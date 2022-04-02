// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"fmt"
	"sync"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/util"
)

type server struct {
	docs map[protocol.DocumentURI]*document

	loader *SchemaCache
}

func Methods(host plugin.Host) *lsp.Methods {
	server := &server{
		docs: map[protocol.DocumentURI]*document{},
		loader: &SchemaCache{
			inner: schema.NewPluginLoader(host),
			m:     &sync.Mutex{},
			cache: map[util.Tuple[string, string]]*schema.Package{},
		},
	}
	return lsp.Methods{
		DidOpenFunc:    server.didOpen,
		DidCloseFunc:   server.didClose,
		DidChangeFunc:  server.didChange,
		HoverFunc:      server.hover,
		CompletionFunc: server.completion,
	}.DefaultInitializer("pulumi-lsp", "0.1.0")
}

func (s *server) setDocument(text lsp.Document) *document {
	doc := &document{text: text, server: s}
	s.docs[text.URI()] = doc
	return doc
}

func (s *server) getDocument(uri protocol.DocumentURI) (*document, bool) {
	d, ok := s.docs[uri]
	return d, ok
}

type document struct {
	text lsp.Document

	server *server

	analysis *documentAnalysisPipeline
}

// Returns a loader. The cancel function must be called when done with the loader.
func (s *server) GetLoader(c lsp.Client) schema.Loader {
	contract.Assert(s.loader.m != nil)
	return SchemaLoader{s.loader, c}
}

func (d *document) process(c lsp.Client) {
	if d.analysis != nil {
		d.analysis.cancel()
	}
	loader := d.server.GetLoader(c)
	d.analysis = NewDocumentAnalysisPipeline(c, d.text, loader)
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
	if err := doc.text.AcceptChanges(params.ContentChanges); err != nil {
		// Something has gone deeply wrong. We rely on having a reliable copy of
		// the document.
		return fmt.Errorf("Document might be unknown: %w", err)
	}
	var defRange protocol.Range
	err := client.LogInfof("%s changed(wholeChange=%t)", fileName, params.ContentChanges[0].Range == defRange)
	doc.process(client)
	return err
}

func (s *server) hover(client lsp.Client, params *protocol.HoverParams) (*protocol.Hover, error) {
	uri := params.TextDocument.URI
	doc, ok := s.getDocument(uri)
	pos := params.Position
	if !ok {
		return nil, fmt.Errorf("Could not find an opened document %s", uri.Filename())
	}
	if doc.analysis == nil {
		panic("Need to implement wait mechanism for docs")
	}
	typ, err := doc.objectAtPoint(pos)
	if err != nil {
		client.LogErrorf(err.Error())
		return nil, nil
	}
	client.LogInfof("Object found for hover: %v", typ)
	if typ != nil {
		if description, ok := typ.Describe(); ok {
			return &protocol.Hover{
				Contents: description,
				Range:    typ.Range(),
			}, nil
		}
	}
	return nil, nil
}

func (s *server) completion(client lsp.Client, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	client.LogWarningf("Completion called")
	uri := params.TextDocument.URI
	doc, ok := s.getDocument(uri)
	if !ok {
		return nil, fmt.Errorf("Could not find an opened document %s", uri.Filename())
	}

	typeFuncCompletion, err := s.completeType(client, doc, params)
	if err != nil {
		return nil, err
	}
	if typeFuncCompletion != nil {
		return typeFuncCompletion, nil
	}

	o, err := doc.objectAtPoint(params.Position)
	if err != nil {
		client.LogErrorf(err.Error())
		return nil, nil
	}
	client.LogInfof("Object found for completion: %v", o)
	// We handle completion when the schema is fully parsed
	if o, ok := o.(*Reference); ok {
		return s.completeReference(client, doc, o)
	}
	return nil, nil
}
