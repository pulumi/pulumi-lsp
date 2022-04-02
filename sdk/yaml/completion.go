// Copyright 2022, Pulumi Corporation.  All rights reserved.
package yaml

import (
	"strings"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/util"
	"github.com/pulumi/pulumi-lsp/sdk/yaml/bind"
)

func (s *server) completeReference(c lsp.Client, doc *document, ref *Reference) (*protocol.CompletionList, error) {
	refTxt, err := doc.text.Window(*ref.Range())
	if err != nil {
		return nil, err
	}
	accessors := ref.ref.Accessors()
	b, ok := doc.analysis.bound.GetResult()
	contract.Assertf(ok, "Should have exited already if the bind task failed")
	if len(accessors) == 0 {
		// We are binding a ref at the top level. We iterate over all top level
		// objects.
		c.LogInfof("Completing %s as reference in the global namespace", refTxt)
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
				Detail:           refTxt,
			})
		}
		return &protocol.CompletionList{Items: list}, nil
	} else if v := ref.ref.Var(); v != nil {
		// We have an associated variable, and are not at the top level. We
		// should try to drill into the variable.
		varType := v.Source().ResolveType(b.A)
		if varType != nil {
			// Don't bother with the error message, it will have already been
			// displayed.
			c.LogInfof("Completing %s as reference to a variable", refTxt)
			if len(accessors) == 1 {
				l, err := s.typePropertyCompletion(varType, v.Name()+".")
				c.LogInfof("One accessor: %v", l)
				return l, err
			} else {
				types, _ := accessors.Typed(varType)
				last := types[len(types)-2]
				if last != nil {
					return s.typePropertyCompletion(last, "")
				}
			}
		}
		c.LogWarningf("Could not complete for %s: could not resolve a variable type", refTxt)
	}
	c.LogWarningf("Could not complete for %s: could not resolve a variable", refTxt)
	return nil, nil
}

func (s *server) typePropertyCompletion(t schema.Type, filterPrefix string) (*protocol.CompletionList, error) {
	switch t := codegen.UnwrapType(t).(type) {
	case *schema.ResourceType:
		r := t.Resource
		l := r.Properties
		if r.InputProperties != nil {
			l = append(l, r.InputProperties...)
		}
		return s.typePropertyFromPropertyList(l, filterPrefix)
	case *schema.ObjectType:
		return s.typePropertyFromPropertyList(t.Properties, filterPrefix)
	default:
		return nil, nil
	}
}

func (s *server) typePropertyFromPropertyList(l []*schema.Property, filterPrefix string) (*protocol.CompletionList, error) {
	items := make([]protocol.CompletionItem, 0, len(l))
	for _, prop := range l {
		typ := codegen.UnwrapType(prop.Type)
		kind := protocol.CompletionItemKindField
		switch typ.(type) {
		case *schema.ResourceType:
			kind = protocol.CompletionItemKindClass
		case *schema.ObjectType:
			kind = protocol.CompletionItemKindStruct
		}
		if typ == schema.StringType {
			kind = protocol.CompletionItemKindText
		}
		items = append(items, protocol.CompletionItem{
			CommitCharacters: []string{".", "["},
			Deprecated:       prop.DeprecationMessage != "",
			FilterText:       filterPrefix + prop.Name,
			InsertText:       prop.Name,
			InsertTextFormat: protocol.InsertTextFormatPlainText,
			InsertTextMode:   protocol.InsertTextModeAsIs,
			Kind:             kind,
			Label:            prop.Name,
		})
	}
	return &protocol.CompletionList{Items: items}, nil
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
