package yaml

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
	parsed     *ast.TemplateDecl
	parseDiags syntax.Diagnostics
	parsedCond *sync.Cond

	// Then the program is analyzed
	bound        *bind.Decl
	boundErrDiag *syntax.Diagnostic
	boundCond    *sync.Cond
}

// Wait for parsed to be populated, then return it. If the parsing fails, it is
// still possible to return a nil pointer.
func (d *documentAnalysisPipeline) GetParsed() *ast.TemplateDecl {
	if d.parsed != nil {
		return d.parsed
	}
	d.parsedCond.L.Lock()
	d.parsedCond.Wait()
	d.parsedCond.L.Unlock()
	return d.parsed
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
	var err error
	d.parsed, d.parseDiags, err = yaml.LoadYAML(text.URI().Filename(), strings.NewReader(text.String()))
	if err != nil {
		d.parseDiags = append(d.parseDiags, d.promoteError("Parse error", err))
		d.cancel()
	} else if d.parsed == nil {
		d.parseDiags = append(d.parseDiags, d.promoteError("Parse error", fmt.Errorf("No template returned")))
		d.cancel()
	}
	d.parsedCond.Broadcast()
}

func (d *documentAnalysisPipeline) bind() {
	var err error
	d.bound, err = bind.NewDecl(d.parsed)
	if err != nil {
		d.boundErrDiag = d.promoteError("Binding error", err)
	}
	d.boundCond.Broadcast()
}

// Wait for bound to be populated, then return it. If the binding fails, it is
// still possible to return a nil pointer.
func (d *documentAnalysisPipeline) GetBound() *bind.Decl {
	if d.bound != nil {
		return d.bound
	}
	d.boundCond.L.Lock()
	d.boundCond.Wait()
	d.boundCond.L.Unlock()
	return d.bound
}

func (d *documentAnalysisPipeline) schematize(l schema.Loader) {
	d.bound.LoadSchema(l)
}

// Creates a new asynchronous analysis pipeline, returning a handle to the process.
func NewDocumentAnalysisPipeline(c lsp.Client, text lsp.Document, loader schema.Loader) *documentAnalysisPipeline {
	ctx, cancel := context.WithCancel(c.Context())
	d := &documentAnalysisPipeline{
		ctx:          ctx,
		cancel:       cancel,
		parsed:       nil,
		parseDiags:   nil,
		parsedCond:   sync.NewCond(&sync.Mutex{}),
		bound:        nil,
		boundErrDiag: nil,
		boundCond:    sync.NewCond(&sync.Mutex{}),
	}

	go func(c lsp.Client, text lsp.Document, loader schema.Loader) {
		// We need to ensure everything finished when we exit
		defer d.cancel()
		free := func(cond *sync.Cond) {
			cond.L.Lock()
			cond.Broadcast()
			cond.L.Unlock()
		}
		defer free(d.parsedCond)
		defer free(d.boundCond)
		c.LogInfof("Kicking off analysis for %s", text.URI().Filename())

		if d.isDone() {
			return
		}
		d.parse(text)
		d.sendDiags(c, text.URI())

		if d.isDone() {
			return
		}
		d.bind()
		d.sendDiags(c, text.URI())

		if d.isDone() {
			return
		}

		d.schematize(loader)
		d.sendDiags(c, text.URI())
	}(c, text, loader)
	return d
}

func (d *documentAnalysisPipeline) diags() syntax.Diagnostics {
	arr := d.parseDiags
	if d.boundErrDiag != nil {
		arr = append(arr, d.boundErrDiag)
	}
	if d.bound != nil {
		arr = append(arr, d.bound.Diags()...)
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
