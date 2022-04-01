// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"

	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/step"
	"github.com/pulumi/pulumi-lsp/sdk/util"
	"github.com/pulumi/pulumi-lsp/sdk/yaml/bind"
)

type documentAnalysisPipeline struct {
	ctx    context.Context
	cancel context.CancelFunc

	// First stage, program is parsed
	parsed *step.Step[util.Tuple[*ast.TemplateDecl, syntax.Diagnostics]]

	// Then the program is analyzed
	bound *step.Step[util.Tuple[*bind.Decl, *syntax.Diagnostic]]
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
	d.parsed = step.New(d.ctx, func() (util.Tuple[*ast.TemplateDecl, syntax.Diagnostics], bool) {
		parsed, parseDiags, err := yaml.LoadYAML(text.URI().Filename(), strings.NewReader(text.String()))
		if err != nil {
			parseDiags = append(parseDiags, d.promoteError("Parse error", err))
		} else if d.parsed == nil {
			parseDiags = append(parseDiags, d.promoteError("Parse error", fmt.Errorf("No template returned")))
		}
		return util.Tuple[*ast.TemplateDecl, syntax.Diagnostics]{A: parsed, B: parseDiags}, true
	})
}

func (d *documentAnalysisPipeline) bind(t util.Tuple[*ast.TemplateDecl, syntax.Diagnostics]) (util.Tuple[*bind.Decl, *syntax.Diagnostic], bool) {
	if t.A == nil {
		return util.Tuple[*bind.Decl, *syntax.Diagnostic]{}, false
	}
	bound, err := bind.NewDecl(t.A)
	var hclErr *hcl.Diagnostic
	if err != nil {
		hclErr = d.promoteError("Binding error", err)
	}
	return util.Tuple[*bind.Decl, *syntax.Diagnostic]{A: bound, B: hclErr}, true
}

// Creates a new asynchronous analysis pipeline, returning a handle to the
// process. To avoid a memory leak, ${RESULT}.cancel must be called.
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
		c.LogInfof("Kicking off analysis for %s", text.URI().Filename())

		d.parse(text)
		step.Then(d.parsed, func(util.Tuple[*ast.TemplateDecl, syntax.Diagnostics]) (struct{}, bool) {
			return struct{}{}, d.sendDiags(c, text.URI()) == nil
		})

		d.bound = step.Then(d.parsed, d.bind)
		step.Then(d.bound, func(util.Tuple[*bind.Decl, *syntax.Diagnostic]) (struct{}, bool) {
			return struct{}{}, d.sendDiags(c, text.URI()) == nil
		})

		schematize := step.Then(d.bound, func(t util.Tuple[*bind.Decl, *syntax.Diagnostic]) (struct{}, bool) {
			if t.A != nil {
				t.A.LoadSchema(loader)
				return struct{}{}, true
			}
			return struct{}{}, false
		})
		step.Then(schematize, func(struct{}) (struct{}, bool) {
			return struct{}{}, d.sendDiags(c, text.URI()) == nil
		})
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
func (d *documentAnalysisPipeline) sendDiags(c lsp.Client, uri protocol.DocumentURI) error {
	lspDiags := []protocol.Diagnostic{}
	for _, diag := range d.diags() {
		if diag == nil {
			continue
		}
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
	return c.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
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
