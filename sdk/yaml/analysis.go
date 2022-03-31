package yaml

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"

	"github.com/iwahbe/pulumi-lsp/sdk/yaml/bind"
	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
)

type documentAnalysisPipeline struct {
	ctx    context.Context
	cancel context.CancelFunc

	// First stage, program is parsed
	parsed *step[Tuple[*ast.TemplateDecl, syntax.Diagnostics]]

	// Then the program is analyzed
	bound *step[Tuple[*bind.Decl, *syntax.Diagnostic]]
}

type step[T any] struct {
	// The returned data, once it is returned.
	data T
	// When this channel is closed, the computation has finished, we we can
	// return the data.
	done chan struct{}
	// If this context is canceled, we return.
	ctx context.Context
}

func (s *step[T]) TryGetResult() (T, bool) {
	var t T
	select {
	case _, _ = <-s.done:
		return s.data, true
	case _, _ = <-s.ctx.Done():
		return t, false
	default:
		return t, false
	}
}

func (s *step[T]) GetResult() (T, bool) {
	var t T
	select {
	case <-s.done:
		return s.data, true
	case <-s.ctx.Done():
		return t, false
	}
}

func NewStep[T any](ctx context.Context, f func() (T, bool)) *step[T] {
	ctx, cancel := context.WithCancel(ctx)
	s := &step[T]{
		ctx:  ctx,
		done: make(chan struct{}),
	}
	go func() {
		var ok bool
		s.data, ok = f()
		if !ok {
			cancel()
		} else {
			close(s.done)
		}
	}()
	return s
}

func StepThen[T any, U any](s *step[T], f func(T) (U, bool)) *step[U] {
	return NewStep(s.ctx, func() (U, bool) {
		var u U
		t, ok := s.GetResult()
		if !ok {
			return u, false
		}
		return f(t)
	})
}

func (d *documentAnalysisPipeline) isDone() bool {
	select {
	case <-d.ctx.Done():
		return true
	default:
		return false
	}
}

// Parse the document
func (d *documentAnalysisPipeline) parse(text lsp.Document) {
	// This is the first step of analysis, so we don't check for previous errors
	d.parsed = NewStep(d.ctx, func() (Tuple[*ast.TemplateDecl, syntax.Diagnostics], bool) {
		parsed, parseDiags, err := yaml.LoadYAML(text.URI().Filename(), strings.NewReader(text.String()))
		if err != nil {
			parseDiags = append(parseDiags, d.promoteError("Parse error", err))
		} else if d.parsed == nil {
			parseDiags = append(parseDiags, d.promoteError("Parse error", fmt.Errorf("No template returned")))
		}
		return Tuple[*ast.TemplateDecl, syntax.Diagnostics]{parsed, parseDiags}, true
	})
}

func (d *documentAnalysisPipeline) bind(t Tuple[*ast.TemplateDecl, syntax.Diagnostics]) (Tuple[*bind.Decl, *syntax.Diagnostic], bool) {
	if t.A == nil {
		return Tuple[*bind.Decl, *syntax.Diagnostic]{}, false
	}
	bound, err := bind.NewDecl(t.A)
	var hclErr *hcl.Diagnostic
	if err != nil {
		hclErr = d.promoteError("Binding error", err)
	}
	return Tuple[*bind.Decl, *syntax.Diagnostic]{bound, hclErr}, true
}

// Creates a new asynchronous analysis pipeline, returning a handle to the process.
func NewDocumentAnalysisPipeline(c lsp.Client, text lsp.Document, loader schema.Loader) *documentAnalysisPipeline {
	ctx, cancel := context.WithCancel(c.Context())
	d := &documentAnalysisPipeline{
		ctx:    ctx,
		cancel: cancel,
		parsed: nil,
		bound:  nil,
	}
	go func(c lsp.Client, text lsp.Document, loader schema.Loader) {
		// We need to ensure everything finished when we exit
		defer cancel()
		c.LogInfof("Kicking off analysis for %s", text.URI().Filename())

		d.parse(text)
		StepThen(d.parsed, func(Tuple[*ast.TemplateDecl, syntax.Diagnostics]) (struct{}, bool) {
			d.sendDiags(c, text.URI())
			return struct{}{}, true
		})

		d.bound = StepThen(d.parsed, d.bind)
		StepThen(d.bound, func(Tuple[*bind.Decl, *syntax.Diagnostic]) (struct{}, bool) {
			d.sendDiags(c, text.URI())
			return struct{}{}, true
		})

		schematize := StepThen(d.bound, func(t Tuple[*bind.Decl, *syntax.Diagnostic]) (struct{}, bool) {
			if t.A != nil {
				t.A.LoadSchema(loader)
				return struct{}{}, true
			}
			return struct{}{}, false
		})
		lastDiag := StepThen(schematize, func(struct{}) (struct{}, bool) {
			d.sendDiags(c, text.URI())
			return struct{}{}, true
		})
		// This will block on `lastDiag`, which will finish only when the entire
		// chain is finished. Blocking prevents the `cancel` from being called
		// prematurely.
		lastDiag.GetResult()
	}(c, text, loader)
	return d
}

func (d *documentAnalysisPipeline) diags() syntax.Diagnostics {
	var arr syntax.Diagnostics
	parsed, ok := d.parsed.TryGetResult()
	if ok && parsed.B != nil {
		arr.Extend(parsed.B...)
	}
	bound, ok := d.bound.TryGetResult()
	if ok {
		if bound.B != nil {
			arr = append(arr, bound.B)
		}
		if bound.A != nil {
			arr = append(arr, bound.A.Diags()...)
		}
	}
	return arr
}

// Actually send the report request to the lsp server
func (d *documentAnalysisPipeline) sendDiags(c lsp.Client, uri protocol.DocumentURI) {
	lspDiags := []protocol.Diagnostic{}
	for _, diag := range d.diags() {
		diagnostic := protocol.Diagnostic{
			Severity: convertSeverity(diag.Severity),
			Source:   "pulumi-yaml",
			Message:  diag.Summary + "\n" + diag.Detail,
		}
		if diag.Subject != nil {
			diagnostic.Range = convertRange(diag.Subject)
		}

		lspDiags = append(lspDiags, diagnostic)
		c.LogInfof("Preparing diagnostic %v", diagnostic)
	}

	// Diagnostics last until the next publish, so we need to publish even if we
	// have not found any diags. This will clear the diags for the user.
	c.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
		URI:         uri,
		Version:     0,
		Diagnostics: lspDiags,
	})
}

func (d *documentAnalysisPipeline) promoteError(msg string, err error) *syntax.Diagnostic {
	return &syntax.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  msg,
		Detail:   err.Error(),
	}
}
