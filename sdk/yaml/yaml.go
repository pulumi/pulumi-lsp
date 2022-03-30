package yaml

import (
	"context"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/hashicorp/hcl/v2"
	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/syntax"
)

type server struct {
	docs map[protocol.DocumentURI]*document

	schemas schemaHandler
}

func Methods() *lsp.Methods {
	server := &server{
		docs:    map[protocol.DocumentURI]*document{},
		schemas: schemaHandler{},
	}
	return lsp.Methods{
		DidOpenFunc:   server.didOpen,
		DidCloseFunc:  server.didClose,
		DidChangeFunc: server.didChange,
		HoverFunc:     server.hover,
	}.DefaultInitializer("pulumi-lsp", "0.1.0")
}

func (s *server) setDocument(text lsp.Document) *document {
	doc := &document{text: text}
	s.docs[text.URI()] = doc
	return doc
}

func (s *server) getDocument(uri protocol.DocumentURI) (*document, bool) {
	d, ok := s.docs[uri]
	return d, ok
}

type document struct {
	text lsp.Document

	analysis *documentAnalysisPipeline
}

type documentAnalysisPipeline struct {
	ctx    context.Context
	cancel context.CancelFunc

	// First stage, program is parsed
	parsed *ast.TemplateDecl
	// Then the program is analyzed
	bound *BoundDecl

	// This is the combined list of diagnostics
	diags syntax.Diagnostics
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
	d.parsed, d.diags, err = yaml.LoadYAML(text.URI().Filename(), strings.NewReader(text.String()))
	if err != nil {
		d.promoteError("Parse error", err)
		d.cancel()
	} else if d.parsed == nil {
		d.promoteError("Parse error", fmt.Errorf("No template returned"))
		d.cancel()
	}
}

func (d *documentAnalysisPipeline) promoteError(msg string, err error) {
	d.diags = append(d.diags, &syntax.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Parse Error",
		Detail:   d.diags.Error(),
	})
}

// Runs the analysis pipeline.
// Should always be called asynchronously.
func (p *documentAnalysisPipeline) kickoff(c lsp.Client, text lsp.Document) {
	// We are resetting the analysis pipeline
	c.LogInfof("Kicking off analysis for %s", text.URI().Filename())
	defer p.cancel()

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
}

func (d *document) process(c lsp.Client) {
	if d.analysis != nil {
		d.analysis.cancel()
	}
	pipe := &documentAnalysisPipeline{}
	pipe.ctx, pipe.cancel = context.WithCancel(c.Context())
	d.analysis = pipe
	go pipe.kickoff(c, d.text)
}

func (d *documentAnalysisPipeline) bind() {
	var err error
	d.bound, err = NewBoundDecl(d.parsed)
	if err != nil {
		d.promoteError("Binding error", err)
	} else {
		d.diags = append(d.diags, d.bound.diags...)
	}
}

// Actually send the report request to the lsp server
func (d *documentAnalysisPipeline) sendDiags(c lsp.Client, uri protocol.DocumentURI) {
	lspDiags := []protocol.Diagnostic{}
	for _, diag := range d.diags {
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

func (s *server) didOpen(client lsp.Client, params *protocol.DidOpenTextDocumentParams) error {
	fileName := params.TextDocument.URI.Filename()
	text := params.TextDocument.Text
	err := client.LogDebugf("Opened file %s:\n---\n%s---", fileName, text)
	s.setDocument(lsp.NewDocument(params.TextDocument)).process(client)
	return err
}

func (s *server) didClose(client lsp.Client, params *protocol.DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI
	client.LogDebugf("Closing file %s", uri.Filename())
	_, ok := s.docs[uri]
	if !ok {
		client.LogWarningf("Attempted to close unopened file %s", uri.Filename())
	}
	delete(s.docs, uri)
	return nil
}

func (s *server) didChange(client lsp.Client, params *protocol.DidChangeTextDocumentParams) error {
	fileName := params.TextDocument.URI.Filename()
	uri := params.TextDocument.URI
	doc, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return fmt.Errorf("could not find document %s(%s)", uri.Filename(), uri)
	}
	doc.text.AcceptChanges(params.ContentChanges)
	var defRange protocol.Range
	err := client.LogInfof("%s changed(wholeChange=%t)", fileName, params.ContentChanges[0].Range == defRange)
	doc.process(client)
	return err
}

func (s *server) hover(client lsp.Client, params *protocol.HoverParams) (*protocol.Hover, error) {
	uri := params.TextDocument.URI
	doc, ok := s.getDocument(uri)
	if !ok {
		return nil, fmt.Errorf("Could not find an opened document %s", uri.Filename())
	}
	if doc.analysis == nil {
		panic("Need to implement wait mechanism for docs")
	}
	typ, location, err := s.schemas.objectAtPoint(doc.analysis.parsed, params.Position)
	if err != nil {
		return nil, fmt.Errorf("Could not find a object at point %v", params.Position)
	} else {
		client.LogInfof("Could not find an object to hover over at %v in %s", params.Position, uri.Filename())
	}
	return &protocol.Hover{
		Contents: describeType(typ),
		Range:    &location,
	}, nil
}
