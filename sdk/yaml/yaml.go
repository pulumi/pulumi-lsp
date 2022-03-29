package yaml

import (
	"strings"

	"go.lsp.dev/protocol"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/syntax"
)

type server struct {
	docs map[protocol.DocumentURI]*document
}

func Methods() *lsp.Methods {
	server := &server{
		docs: map[protocol.DocumentURI]*document{},
	}
	return lsp.Methods{
		DidOpenFunc:   server.didOpen,
		DidChangeFunc: server.didChange,
	}.DefaultInitializer("pulumi-lsp", "0.1.0")
}

type document struct {
	uri    protocol.DocumentURI
	text   string
	parsed *ast.TemplateDecl
	diags  syntax.Diagnostics
	err    error
}

func (s *server) setDocument(client lsp.Client, uri protocol.DocumentURI, text string) {
	doc := &document{text: text, uri: uri}
	s.docs[uri] = doc
	go func() {
		doc.parse()
		doc.reportDiags(client)
	}()

}

// Parse the document
func (d *document) parse() {
	d.parsed, d.diags, d.err = yaml.LoadYAML(d.uri.Filename(), strings.NewReader(d.text))
}

// Report any diagnostics to the server
func (d *document) reportDiags(client lsp.Client) {
	diags := []protocol.Diagnostic{}

	if d.err != nil {
		client.LogErrorf("Parsing error: %s", d.err.Error())
	} else if d.diags != nil {
		for _, diag := range d.diags {

			diags = append(diags, protocol.Diagnostic{
				Range:              convertRange(diag.Subject),
				Severity:           convertSeverity(diag.Severity),
				Source:             "pulumi-yaml",
				Message:            diag.Summary + "\n" + diag.Detail,
				RelatedInformation: []protocol.DiagnosticRelatedInformation{},
			})
		}
	}

	if len(diags) > 0 {
		client.PublishDiagnostics(&protocol.PublishDiagnosticsParams{
			URI:         d.uri,
			Version:     0,
			Diagnostics: diags,
		})
	}
}
func (s *server) didOpen(client lsp.Client, params *protocol.DidOpenTextDocumentParams) error {
	fileName := params.TextDocument.URI.Filename()
	text := params.TextDocument.Text
	err := client.LogDebugf("Opened file %s:\n---\n%s---", fileName, text)
	s.setDocument(client, params.TextDocument.URI, text)
	return err
}

func (s *server) didChange(client lsp.Client, params *protocol.DidChangeTextDocumentParams) error {
	fileName := params.TextDocument.URI.Filename()
	err := client.LogDebugf("%s changed", fileName)
	s.setDocument(client, params.TextDocument.URI, params.ContentChanges[0].Text)
	return err
}
