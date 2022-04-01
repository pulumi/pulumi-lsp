package yaml

import (
	"fmt"
	"strings"
	"sync"

	"go.lsp.dev/protocol"

	"github.com/iwahbe/pulumi-lsp/sdk/lsp"
	"github.com/iwahbe/pulumi-lsp/sdk/yaml/bind"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/iwahbe/pulumi-lsp/sdk/util"
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
	doc.text.AcceptChanges(params.ContentChanges)
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
		ref, err := doc.text.Window(*o.Range())
		if err != nil {
			return nil, err
		}
		// We are binding a ref that doesn't have an associated variable,
		b, ok := doc.analysis.bound.GetResult()
		contract.Assertf(ok, "Should have exited already if the bind task failed")
		list := []protocol.CompletionItem{}
		for k, v := range b.A.Variables() {
			kind := protocol.CompletionItemKindVariable
			if _, ok := v.Source().(*bind.Resource); ok {
				kind = protocol.CompletionItemKindClass
			}
			list = append(list, protocol.CompletionItem{
				CommitCharacters: []string{"."},
				InsertTextFormat: protocol.InsertTextFormatPlainText,
				InsertTextMode:   protocol.InsertTextModeAsIs,
				Kind:             kind,
				Label:            k,
				Detail:           ref,
			})
		}
		return &protocol.CompletionList{Items: list}, nil
	}
	return nil, nil
}

// We handle type completion without relying on a parsed schema. This is because that dangling `:` cause parse failures.
// So `Type: ` and `Type: eks:` are all invalid.
func (s *server) completeType(client lsp.Client, doc *document, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	pos := params.Position
	line, err := doc.text.Line(int(pos.Line))
	if err != nil {
		return nil, err
	}
	window, err := doc.text.Window(line)
	if err != nil {
		return nil, err
	}
	window = strings.TrimSpace(window)

	if strings.HasPrefix(window, "type:") {
		currentWord := strings.TrimSpace(strings.TrimPrefix(window, "type:"))
		// Don't know that this is, just dump out
		if strings.Contains(currentWord, " \t") {
			client.LogDebugf("To many spaces in current word: %#v", currentWord)
			return nil, nil
		}
		parts := strings.Split(currentWord, ":")
		client.LogInfof("Completing resource type: %v", parts)
		switch len(parts) {
		case 1:
			doPad := strings.TrimPrefix(window, "type:")
			return s.pkgCompletionList(doPad == ""), nil
		case 2:
			pkg, err := s.GetLoader(client).LoadPackage(parts[0], nil)
			if err != nil {
				return nil, err
			}
			mods := moduleCompletionList(pkg)
			client.LogDebugf("Found %d modules for %s", len(mods), pkg.Name)
			return &protocol.CompletionList{
				Items: append(mods, resourceCompletionList(pkg, "")...),
			}, nil
		case 3:
			pkg, err := s.GetLoader(client).LoadPackage(parts[0], nil)
			if err != nil {
				return nil, err
			}
			return &protocol.CompletionList{
				Items: resourceCompletionList(pkg, parts[1]),
			}, nil
		default:
			client.LogDebugf("Found too many words to complete")
			return nil, nil
		}
	}

	if strings.HasPrefix(window, "Function:") {
		client.LogWarningf("Function type completion not implemented yet")
	}

	return nil, nil
}

// Return the list of currently loaded packages.
func (s *server) pkgCompletionList(pad bool) *protocol.CompletionList {
	s.loader.m.Lock()
	defer s.loader.m.Unlock()
	return &protocol.CompletionList{
		Items: util.MapOver(util.MapValues(s.loader.cache), func(p *schema.Package) protocol.CompletionItem {
			insert := p.Name + ":"
			if pad {
				insert = " " + insert
			}
			return protocol.CompletionItem{
				CommitCharacters: []string{":"},
				Documentation:    p.Description,
				FilterText:       p.Name,
				InsertText:       insert,
				InsertTextFormat: protocol.InsertTextFormatPlainText,
				InsertTextMode:   protocol.InsertTextModeAsIs,
				Kind:             protocol.CompletionItemKindModule,
				Label:            p.Name,
			}
		}),
	}
}

func moduleCompletionList(pkg *schema.Package) []protocol.CompletionItem {
	m := map[string]bool{}
	for _, r := range pkg.Resources {
		s := pkg.TokenToModule(r.Token)
		m[s] = m[s] || r.DeprecationMessage != ""
	}
	out := make([]protocol.CompletionItem, 0, len(m))
	for mod, depreciated := range m {
		full := pkg.Name + ":" + mod
		out = append(out, protocol.CompletionItem{
			CommitCharacters: []string{":"},
			Deprecated:       depreciated,
			FilterText:       full,
			InsertText:       mod + ":",
			InsertTextFormat: protocol.InsertTextFormatPlainText,
			InsertTextMode:   protocol.InsertTextModeAsIs,
			Kind:             protocol.CompletionItemKindModule,
			Label:            mod,
		})
	}
	return out
}

func resourceCompletionList(pkg *schema.Package, modHint string) []protocol.CompletionItem {
	out := []protocol.CompletionItem{}
	for _, r := range pkg.Resources {
		mod := pkg.TokenToModule(r.Token)
		parts := strings.Split(r.Token, ":")
		name := parts[len(parts)-1]

		// We want to use modHint as a prefix (a weak fuzzy filter), but only if
		// modHint is "". When modHint is "", we interpret it as looking at the
		// "index" module, so we need exact matches.
		if (strings.HasPrefix(mod, modHint) && modHint != "") || mod == modHint {
			out = append(out, protocol.CompletionItem{
				Deprecated:       r.DeprecationMessage != "",
				FilterText:       r.Token,
				InsertText:       name,
				InsertTextFormat: protocol.InsertTextFormatPlainText,
				InsertTextMode:   protocol.InsertTextModeAsIs,
				Kind:             protocol.CompletionItemKindClass,
				Label:            name,
			})
		}
	}
	return out
}
