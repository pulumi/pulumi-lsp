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
	parsed     *ast.TemplateDecl
	parseDiags syntax.Diagnostics
	// Then the program is analyzed
	bound       *bind.Decl
	bindErrDiag *syntax.Diagnostic
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
}

func (d *documentAnalysisPipeline) bind() {
	var err error
	d.bound, err = bind.NewDecl(d.parsed)
	if err != nil {
		d.bindErrDiag = d.promoteError("Binding error", err)
	}
}

func (d *documentAnalysisPipeline) schematize(l schema.Loader) {
	d.bound.LoadSchema(l)
}

// Runs the analysis pipeline.
// Should always be called asynchronously.
func (p *documentAnalysisPipeline) kickoff(c lsp.Client, text lsp.Document, loader schema.Loader, onDone func()) {
	defer onDone()
	defer p.cancel()
	// We are resetting the analysis pipeline
	c.LogInfof("Kicking off analysis for %s", text.URI().Filename())
	p.bindErrDiag = nil
	p.bound = nil
	p.parseDiags = nil
	p.parsed = nil

	if p.isDone() {
		return
	}
	p.parse(text)
	p.sendDiags(c, text.URI())

	if p.isDone() {
		return
	}
	p.bind()
	p.sendDiags(c, text.URI())

	if p.isDone() {
		return
	}

	p.schematize(loader)
	p.sendDiags(c, text.URI())
}

func (d *documentAnalysisPipeline) diags() syntax.Diagnostics {
	arr := d.parseDiags
	if d.bindErrDiag != nil {
		arr = append(arr, d.bindErrDiag)
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
