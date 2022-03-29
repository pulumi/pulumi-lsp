package yaml

import (
	"go.lsp.dev/protocol"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
)

type server struct {
	docs map[protocol.DocumentURI]*document
}

type document struct {
	text string
}

func newDocument(text string) *document {
	return &document{text: text}
}

func Methods() *lsp.Methods {
	server := &server{
		docs: map[protocol.DocumentURI]*document{},
	}
	return lsp.Methods{
		DidOpenFunc: server.didOpen,
	}.DefaultInitializer("pulumi-lsp", "0.1.0")
}

func (s *server) didOpen(client lsp.Client, params *protocol.DidOpenTextDocumentParams) error {
	err := client.LogDebugf("Opened file %s:\n---\n%s---", params.TextDocument.URI.Filename(), params.TextDocument.Text)
	s.docs[params.TextDocument.URI] = newDocument(params.TextDocument.Text)
	return err
}
