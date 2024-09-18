// Copyright 2022, Pulumi Corporation.  All rights reserved.

// The logic specific to Pulumi YAML.
package yaml

import (
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/version"
	"github.com/pulumi/pulumi-lsp/sdk/yaml/util/loader"
)

// The holder server level state.
type server struct {
	docs    map[protocol.DocumentURI]*document
	schemas loader.ReferenceLoader
}

// Create the set of methods necessary to implement a LSP server for Pulumi YAML.
func Methods(host plugin.Host) *lsp.Methods {
	server := &server{
		docs:    map[protocol.DocumentURI]*document{},
		schemas: loader.New(host),
	}
	return lsp.Methods{
		DidOpenFunc:    server.didOpen,
		DidCloseFunc:   server.didClose,
		DidChangeFunc:  server.didChange,
		HoverFunc:      server.hover,
		CompletionFunc: server.completion,
	}.DefaultInitializer("pulumi-lsp", version.Version)
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

// The representation of a document as used by the server.
type document struct {
	// The actual text of the document.
	text lsp.Document

	// A back-link to the server
	server *server

	// A handle to the currently executing analysis pipeline.
	analysis *documentAnalysisPipeline
}

// Starts an analysis process for the document.
func (d *document) process(c lsp.Client) {
	if d.analysis != nil {
		d.analysis.cancel()
	}
	d.analysis = NewDocumentAnalysisPipeline(c, d.text, d.server.schemas)
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
	doc.process(client)
	return nil
}

func (s *server) hover(client lsp.Client, params *protocol.HoverParams) (*protocol.Hover, error) {
	uri := params.TextDocument.URI
	doc, ok := s.getDocument(uri)
	pos := params.Position
	if !ok {
		return nil, fmt.Errorf("Could not find an opened document %s", uri.Filename())
	}
	if doc.analysis == nil {
		// Do nothing. We can try again later.
		return nil, nil
	}
	typ, err := doc.objectAtPoint(pos)
	if err != nil {
		client.LogErrorf("%s", err.Error())
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

	// Complete for `type: ...` or `Function: ...`.
	typeFuncCompletion, err := s.completeType(client, doc, params)
	if err != nil || typeFuncCompletion != nil {
		return typeFuncCompletion, err
	}

	// Complete for new keys in the YAML
	keyCompletion, err := s.completeKey(client, doc, params)
	if err != nil || keyCompletion != nil {
		return keyCompletion, err
	}

	o, err := doc.objectAtPoint(params.Position)
	if err != nil {
		client.LogErrorf("%s", err.Error())
		return nil, nil
	}
	client.LogInfof("Object found for completion: %v", o)
	// We handle completion when the schema is fully parsed
	if o, ok := o.(*Reference); ok {
		return s.completeReference(client, doc, o)
	}
	client.LogWarningf("No handler responded to completion call")
	return nil, nil
}
