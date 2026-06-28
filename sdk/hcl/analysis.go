// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"context"

	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"

	hclast "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/ast"
	hclparser "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/parser"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/step"
	"github.com/pulumi/pulumi-lsp/sdk/util"
)

// documentAnalysisPipeline runs the staged analysis of a single HCL document.
//
// M1 implements the first stage: parse the document into a *ast.Config and
// publish the resulting hcl.Diagnostics. The bind and schema stages (M2) hang
// off d.parsed the same way sdk/yaml/analysis.go chains its stages.
type documentAnalysisPipeline struct {
	ctx    context.Context
	cancel context.CancelFunc

	// First stage: the program is parsed into an HCL AST.
	parsed *step.Step[util.Tuple[*hclast.Config, hcl.Diagnostics]]
}

// newDocumentAnalysisPipeline kicks off asynchronous analysis of the document.
// To avoid a leak, the returned pipeline's cancel must eventually be called
// (the document handler does this when it starts a fresh pipeline).
func newDocumentAnalysisPipeline(c lsp.Client, text lsp.Document) *documentAnalysisPipeline {
	ctx, cancel := context.WithCancel(c.Context())
	d := &documentAnalysisPipeline{ctx: ctx, cancel: cancel}

	go func() {
		c.LogDebugf("Kicking off HCL analysis for %s", text.URI().Filename())

		d.parse(text)
		step.After(d.parsed, func(util.Tuple[*hclast.Config, hcl.Diagnostics]) {
			contract.IgnoreError(d.sendDiags(c, text.URI()))
		})
	}()

	return d
}

// parse runs the HCL parser over the in-memory document text. The pulumi-hcl
// parser returns a partial *ast.Config alongside any syntactic/structural
// diagnostics, so both are retained for downstream stages.
func (d *documentAnalysisPipeline) parse(text lsp.Document) {
	d.parsed = step.New(d.ctx, func() (util.Tuple[*hclast.Config, hcl.Diagnostics], bool) {
		p := hclparser.NewParser()
		config, diags := p.ParseSource(text.URI().Filename(), []byte(text.String()))
		return util.Tuple[*hclast.Config, hcl.Diagnostics]{A: config, B: diags}, true
	})
}

// diags collects every diagnostic produced by the completed stages.
func (d *documentAnalysisPipeline) diags() hcl.Diagnostics {
	var arr hcl.Diagnostics
	if d.parsed != nil {
		if parsed, ok := d.parsed.TryGetResult(); ok {
			arr = append(arr, parsed.B...)
		}
	}
	return arr
}

// toLSPDiagnostics converts HCL diagnostics into their LSP representation.
func toLSPDiagnostics(diags hcl.Diagnostics) []protocol.Diagnostic {
	lspDiags := []protocol.Diagnostic{}
	for _, diag := range diags {
		if diag == nil {
			continue
		}
		diagnostic := protocol.Diagnostic{
			Severity: convertSeverity(diag.Severity),
			Source:   "pulumi-hcl",
			Message:  diagMessage(diag),
		}
		if diag.Subject != nil {
			diagnostic.Range = convertRange(diag.Subject)
		}
		lspDiags = append(lspDiags, diagnostic)
	}
	return lspDiags
}

// sendDiags converts and publishes the current diagnostics. It publishes even
// when empty so that a previously-broken document is cleared once it parses.
func (d *documentAnalysisPipeline) sendDiags(c lsp.Client, uri protocol.DocumentURI) error {
	return c.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
		URI:         uri,
		Version:     0,
		Diagnostics: toLSPDiagnostics(d.diags()),
	})
}
