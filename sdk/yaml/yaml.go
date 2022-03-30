package yaml

import (
	"context"
	"fmt"
	"sync"

	"go.lsp.dev/protocol"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

type server struct {
	docs map[protocol.DocumentURI]*document

	loader   schema.Loader
	loaderMx sync.Mutex
}

func Methods(host plugin.Host) *lsp.Methods {
	server := &server{
		docs:   map[protocol.DocumentURI]*document{},
		loader: schema.NewPluginLoader(host),
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

	server server

	analysis *documentAnalysisPipeline
}

// Returns a loader. The cancel function must be called when done with the loader.
func (s *server) GetLoader(c lsp.Client) (schema.Loader, func()) {
	s.loaderMx.Lock()
	return LogSchemaLoader{s.loader, c}, func() { s.loaderMx.Lock() }
}

func (d *document) process(c lsp.Client) {
	if d.analysis != nil {
		d.analysis.cancel()
	}
	pipe := &documentAnalysisPipeline{}
	pipe.ctx, pipe.cancel = context.WithCancel(c.Context())
	d.analysis = pipe
	loader, unlock := d.server.GetLoader(c)
	go pipe.kickoff(c, d.text, loader, unlock)
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
	typ, location, err := objectAtPoint(doc.analysis.parsed, params.Position)
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
