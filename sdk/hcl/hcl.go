// Copyright 2026, Pulumi Corporation.  All rights reserved.

// The logic specific to Pulumi HCL (Terraform-syntax) programs.
//
// This package mirrors the structure of the sibling `sdk/yaml` package: a
// `server` holds per-document state, `Methods` wires the LSP callbacks, and each
// open document runs an analysis pipeline that publishes diagnostics.
//
// M0 status: this is the wiring skeleton. The handlers track open documents and
// run a no-op analysis pipeline so the `--lang hcl` server can be launched and
// driven by an editor end-to-end. The real parse/bind/schema stages (built on
// the pulumi-labs/pulumi-hcl library) land in M1+.
package hcl

import (
	"fmt"
	"sync"

	"go.lsp.dev/protocol"

	hclast "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/ast"
	"github.com/pulumi-labs/pulumi-hcl/pkg/hcl/bridge"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/version"
)

// The holder of server-level state.
type server struct {
	docs map[protocol.DocumentURI]*document

	// loader resolves Pulumi package schemas from installed plugins.
	loader schema.ReferenceLoader

	// infoSource provides Terraform-bridge mappings, built in-process from the
	// plugin host. It may be nil, in which case resolution falls back to the
	// convention-based path.
	infoSource bridge.ProviderInfoSource

	// warnedProviders records providers we have already hinted about (e.g. not
	// installed), so the notice is logged once per session rather than on every
	// keystroke.
	warnedProviders sync.Map
}

// Methods creates the set of methods necessary to implement an LSP server for
// Pulumi HCL. It deliberately mirrors yaml.Methods so the two backends stay
// structurally aligned.
func Methods(pctx *plugin.Context) *lsp.Methods {
	server := &server{
		docs:       map[protocol.DocumentURI]*document{},
		loader:     schema.NewPluginLoader(pctx),
		infoSource: newProviderInfoSource(pctx),
	}
	return lsp.Methods{
		DidOpenFunc:        server.didOpen,
		DidCloseFunc:       server.didClose,
		DidChangeFunc:      server.didChange,
		HoverFunc:          server.hover,
		CompletionFunc:     server.completion,
		DefinitionFunc:     server.definition,
		DocumentSymbolFunc: server.documentSymbol,
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

	// A back-link to the server.
	server *server

	// A handle to the currently executing analysis pipeline.
	analysis *documentAnalysisPipeline
}

// Starts (or restarts) an analysis process for the document.
func (d *document) process(c lsp.Client) {
	if d.analysis != nil {
		d.analysis.cancel()
	}
	d.analysis = newDocumentAnalysisPipeline(c, d.text)
}

func (s *server) didOpen(client lsp.Client, params *protocol.DidOpenTextDocumentParams) error {
	fileName := params.TextDocument.URI.Filename()
	text := params.TextDocument.Text
	err := client.LogDebugf("Opened HCL file %s:\n---\n%s---", fileName, text)
	s.setDocument(lsp.NewDocument(params.TextDocument)).process(client)
	return err
}

func (s *server) didClose(client lsp.Client, params *protocol.DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI
	client.LogDebugf("Closing HCL file %s", uri.Filename())
	if _, ok := s.docs[uri]; !ok {
		client.LogWarningf("Attempted to close unopened file %s", uri.Filename())
	}
	delete(s.docs, uri)
	return nil
}

func (s *server) didChange(client lsp.Client, params *protocol.DidChangeTextDocumentParams) error {
	uri := params.TextDocument.URI
	doc, ok := s.getDocument(uri)
	if !ok {
		return fmt.Errorf("could not find document %s(%s)", uri.Filename(), uri)
	}
	if err := doc.text.AcceptChanges(params.ContentChanges); err != nil {
		return fmt.Errorf("document might be unknown: %w", err)
	}
	doc.process(client)
	return nil
}

func (s *server) hover(client lsp.Client, params *protocol.HoverParams) (*protocol.Hover, error) {
	_, target, config, ok := s.targetAtPoint(params.TextDocument.URI, params.Position)
	if !ok {
		return nil, nil
	}

	ctx := client.Context()
	r := target.resource
	ref := s.providerRef(params.TextDocument.URI, config, r.Type)

	// Resolve the schema and bridge mapping for the block. We collect the input
	// properties (for attribute hover) and, when the cursor is on the type
	// label, the full block description.
	var (
		inputs []*schema.Property
		bm     *bridge.BodyMapping
	)
	if r.IsDataSource {
		fn, err := s.resolveFunction(ctx, ref, r.Type)
		if err != nil || fn == nil {
			s.warnUnresolved(client, ref, err)
			return nil, nil
		}
		bm = s.dataSourceBodyMapping(ctx, ref, r.Type)
		if fn.Inputs != nil {
			inputs = fn.Inputs.Properties
		}
		if target.onTypeLabel {
			rng := convertRange(&r.TypeRange)
			return &protocol.Hover{Contents: describeFunction(fn, bm), Range: &rng}, nil
		}
	} else {
		res, err := s.resolveResource(ctx, ref, r.Type)
		if err != nil || res == nil {
			s.warnUnresolved(client, ref, err)
			return nil, nil
		}
		bm = s.resourceBodyMapping(ctx, ref, r.Type)
		inputs = res.InputProperties
		if target.onTypeLabel {
			rng := convertRange(&r.TypeRange)
			return &protocol.Hover{Contents: describeResource(res, bm), Range: &rng}, nil
		}
	}

	// In the body: hover over an attribute name shows that property's details.
	attrName, attrRange, found := attributeAtPoint(r, params.Position)
	if !found {
		return nil, nil
	}
	prop := findProperty(inputs, fieldNames(bm), attrName)
	if prop == nil {
		return nil, nil
	}
	rng := convertRange(&attrRange)
	return &protocol.Hover{Contents: describeProperty(prop, attrName), Range: &rng}, nil
}

func (s *server) completion(client lsp.Client, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	// Resource/data type-name completion is line-based so it works even while
	// the half-typed block does not parse.
	if doc, ok := s.getDocument(params.TextDocument.URI); ok {
		if rt, err := s.completeResourceType(client, doc, params); err != nil || rt != nil {
			return rt, err
		}
	}

	_, target, config, ok := s.targetAtPoint(params.TextDocument.URI, params.Position)
	if !ok || target.onTypeLabel {
		return nil, nil
	}

	// Inside a resource/data-source body: offer the schema's input properties,
	// labelled with their Terraform field names from the bridge mapping.
	ctx := client.Context()
	r := target.resource
	ref := s.providerRef(params.TextDocument.URI, config, r.Type)

	var (
		inputs []*schema.Property
		bm     *bridge.BodyMapping
	)
	if r.IsDataSource {
		fn, err := s.resolveFunction(ctx, ref, r.Type)
		if err != nil || fn == nil || fn.Inputs == nil {
			s.warnUnresolved(client, ref, err)
			return nil, nil
		}
		inputs = fn.Inputs.Properties
		bm = s.dataSourceBodyMapping(ctx, ref, r.Type)
	} else {
		res, err := s.resolveResource(ctx, ref, r.Type)
		if err != nil || res == nil {
			s.warnUnresolved(client, ref, err)
			return nil, nil
		}
		inputs = res.InputProperties
		bm = s.resourceBodyMapping(ctx, ref, r.Type)
	}

	names := fieldNames(bm)
	items := make([]protocol.CompletionItem, 0, len(inputs))
	for _, p := range inputs {
		detail := codegen.UnwrapType(p.Type).String()
		item := protocol.CompletionItem{
			Label:  tfFieldName(names, p.Name),
			Kind:   protocol.CompletionItemKindField,
			Detail: detail,
		}
		if p.Comment != "" {
			item.Documentation = firstLine(p.Comment)
		}
		items = append(items, item)
	}
	return &protocol.CompletionList{Items: items}, nil
}

// targetAtPoint resolves the open document, its parsed config, and the AST node
// under pos. ok is false when any of those are unavailable.
func (s *server) targetAtPoint(
	uri protocol.DocumentURI, pos protocol.Position,
) (*document, *pointTarget, *hclast.Config, bool) {
	doc, ok := s.getDocument(uri)
	if !ok || doc.analysis == nil || doc.analysis.parsed == nil {
		return nil, nil, nil, false
	}
	parsed, ok := doc.analysis.parsed.GetResult()
	if !ok || parsed.A == nil {
		return nil, nil, nil, false
	}
	target := objectAtPoint(parsed.A, pos)
	if target == nil {
		return nil, nil, nil, false
	}
	return doc, target, parsed.A, true
}

// providerRef resolves the provider for a type, honoring the program's declared
// `source` and any sibling .terraform.lock.hcl.
func (s *server) providerRef(uri protocol.DocumentURI, config *hclast.Config, tfType string) providerRef {
	lock := readLockfile(dirForURI(string(uri)))
	return providerFor(config, lock, tfType)
}

// warnUnresolved surfaces a one-time hint when a provider with a declared source
// cannot be resolved — most often because its plugin is not installed.
func (s *server) warnUnresolved(client lsp.Client, ref providerRef, err error) {
	if ref.source == "" {
		return // inferred provider; nothing actionable to say
	}
	if _, seen := s.warnedProviders.LoadOrStore("src:"+ref.source, true); seen {
		return
	}
	client.LogInfof("pulumi-hcl: could not resolve provider %q (source %q) — is its plugin installed? "+
		"Try `pulumi plugin install resource %s`. (%v)", ref.pkg, ref.source, ref.pkg, err)
}

// configFor returns the parsed config for an open document, if available.
func (s *server) configFor(uri protocol.DocumentURI) (*hclast.Config, bool) {
	doc, ok := s.getDocument(uri)
	if !ok || doc.analysis == nil || doc.analysis.parsed == nil {
		return nil, false
	}
	parsed, ok := doc.analysis.parsed.GetResult()
	if !ok || parsed.A == nil {
		return nil, false
	}
	return parsed.A, true
}
