// Copyright 2022, Pulumi Corporation.  All rights reserved.
package yaml

import (
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/util"
)

// Tries to provide completion withing symbols and references.
func (s *server) completeReference(c lsp.Client, doc *document, ref *Reference) (*protocol.CompletionList, error) {
	refTxt, err := doc.text.Window(*ref.Range())
	if err != nil {
		return nil, err
	}
	accessors := ref.ref.Accessors()
	b, ok := doc.analysis.bound.GetResult()
	contract.Assertf(ok, "Should have exited already if the bind task failed")

	// We go through this song and dance to figure out if a property access list
	// ends in a "."
	plainReference := strings.TrimPrefix(refTxt, "${")
	plainReference = strings.TrimSuffix(plainReference, "}")
	if len(accessors) == 0 && !strings.HasSuffix(plainReference, ".") {
		// We are binding a ref at the top level. We iterate over all top level
		// objects.
		c.LogInfof("Completing %s as reference in the global namespace", refTxt)
		list := []protocol.CompletionItem{}
		for k, v := range b.A.Variables() {
			var typ schema.Type
			if v != nil {
				if s := v.Source(); s != nil {
					typ = s.ResolveType(b.A)
				}
			}
			item := completionItemFromType(typ)
			item.CommitCharacters = []string{"."}
			item.InsertTextFormat = protocol.InsertTextFormatPlainText
			item.InsertTextMode = protocol.InsertTextModeAsIs
			item.Label = k

			list = append(list, item)
		}
		return &protocol.CompletionList{Items: list}, nil
	} else if v := ref.ref.Var(); v != nil {
		// We have an associated variable, and are not at the top level. We
		// should try to drill into the variable.
		varRoot := v.Source().ResolveType(b.A)
		if varRoot != nil {
			// Don't bother with the error message, it will have already been
			// displayed.
			c.LogInfof("Completing %s as reference to a variable (%s)", refTxt, ref.ref.Var().Name())
			types, _ := accessors.TypeFromRoot(varRoot)
			c.LogInfof("Found types: %v", types)
			// Return a completion generated from the last type
			return s.typePropertyCompletion(types[len(types)-1], "")
		}
		c.LogWarningf("Could not complete for %s: could not resolve a variable type", refTxt)
	}
	c.LogWarningf("Could not complete for %s: could not resolve a variable", refTxt)
	return nil, nil
}

// Completion for the properties of a type. `filterPrefix` is pre-appended to
// all results to prevent the filter from being eaten by the host.
//
// For example:
//   typePropertyCompletion(type({foo: string, bar: int}), "someType.") would
//   complete to ["someType.foo", "someType.bar"].
func (s *server) typePropertyCompletion(t schema.Type, filterPrefix string) (*protocol.CompletionList, error) {
	switch t := codegen.UnwrapType(t).(type) {
	case *schema.ResourceType:
		r := t.Resource
		l := r.Properties
		if r.InputProperties != nil {
			l = append(l, r.InputProperties...)
		}
		return s.propertyListCompletion(l, filterPrefix)
	case *schema.ObjectType:
		return s.propertyListCompletion(t.Properties, filterPrefix)
	default:
		return nil, nil
	}
}

// Returns the completion option for a property list. filterPrefix is pre-appended to the filter property of all results.
func (s *server) propertyListCompletion(l []*schema.Property, filterPrefix string) (*protocol.CompletionList, error) {
	items := make([]protocol.CompletionItem, 0, len(l))
	for _, prop := range l {
		// Derive item from type
		item := completionItemFromType(prop.Type)
		// then set property level defaults
		item.CommitCharacters = []string{".", "["}
		item.Deprecated = prop.DeprecationMessage != ""
		item.FilterText = filterPrefix + prop.Name
		item.InsertText = prop.Name
		item.InsertTextFormat = protocol.InsertTextFormatPlainText
		item.InsertTextMode = protocol.InsertTextModeAsIs
		item.Label = prop.Name

		items = append(items, item)
	}
	return &protocol.CompletionList{Items: items}, nil
}

func completionItemFromType(t schema.Type) protocol.CompletionItem {
	t = codegen.UnwrapType(t)
	switch t {
	case schema.StringType:
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindText,
			Detail: "String",
		}
	case schema.ArchiveType:
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindFile,
			Detail: "Archive",
		}
	case schema.AssetType:
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindFile,
			Detail: "Asset",
		}
	case schema.BoolType:
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindValue,
			Detail: "Boolean",
		}
	case schema.IntType:
		fallthrough
	case schema.NumberType:
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindValue,
			Detail: "Number",
		}
	case schema.AnyType:
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindValue,
			Detail: "Any",
		}
	}
	switch t := t.(type) {
	case *schema.ResourceType:
		var documentation string
		if t.Resource != nil {
			documentation = t.Resource.Comment
		}
		return protocol.CompletionItem{
			Kind:          protocol.CompletionItemKindClass,
			Detail:        fmt.Sprintf("resource %s", t.Token),
			Documentation: documentation,
		}
	case *schema.ObjectType:
		return protocol.CompletionItem{
			Kind:          protocol.CompletionItemKindInterface,
			Detail:        fmt.Sprintf("object %s", t.Token),
			Documentation: t.Comment,
		}
	case *schema.UnionType:
		var detail string
		if len(t.ElementTypes) == 0 {
			detail = completionItemFromType(t.ElementTypes[0]).Detail
			for i := 1; i < len(t.ElementTypes); i++ {
				detail += "| " + completionItemFromType(t.ElementTypes[i]).Detail
			}
		}
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindValue,
			Detail: detail,
		}
	case *schema.EnumType:
		return protocol.CompletionItem{
			Kind:   protocol.CompletionItemKindEnum,
			Detail: fmt.Sprintf("enum %s", t.Token),
		}
	case *schema.ArrayType:
		inner := completionItemFromType(t.ElementType)
		inner.Detail = fmt.Sprintf("List<%s>", inner.Detail)
		inner.Kind = protocol.CompletionItemKindVariable
		return inner
	case *schema.MapType:
		inner := completionItemFromType(t.ElementType)
		inner.Detail = fmt.Sprintf("Map<%s>", inner.Detail)
		inner.Kind = protocol.CompletionItemKindVariable
		return inner
	default:
		return protocol.CompletionItem{}
	}
}

// We handle type completion without relying on a parsed schema. This is because
// that dangling `:` causes parse failures. `Type: ` and `Type: eks:` are all
// invalid.
func (s *server) completeType(client lsp.Client, doc *document, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	pos := params.Position
	line, err := doc.text.Line(int(pos.Line))
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)

	handleType := func(prefix string, resolve func(*schema.Package, string) []protocol.CompletionItem) (*protocol.CompletionList, error) {
		if strings.HasPrefix(line, prefix) {
			currentWord := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			// Don't know that this is, just dump out
			if strings.Contains(currentWord, " \t") {
				client.LogDebugf("To many spaces in current word: %#v", currentWord)
				return nil, nil
			}
			parts := strings.Split(currentWord, ":")
			client.LogInfof("Completing %v", parts)
			switch len(parts) {
			case 1:
				doPad := strings.TrimPrefix(line, prefix)
				return s.pkgCompletionList(doPad == ""), nil
			case 2:
				pkg, err := s.schemas.Loader(client).LoadPackage(parts[0], nil)
				if err != nil {
					return nil, err
				}
				mods := moduleCompletionList(pkg)
				client.LogDebugf("Found %d modules for %s", len(mods), pkg.Name)
				return &protocol.CompletionList{
					Items: append(mods, resolve(pkg, "")...),
				}, nil
			case 3:
				pkg, err := s.schemas.Loader(client).LoadPackage(parts[0], nil)
				if err != nil {
					return nil, err
				}
				return &protocol.CompletionList{
					Items: resolve(pkg, parts[1]),
				}, nil
			default:
				client.LogDebugf("Found too many words to complete")
				return nil, nil
			}
		}
		return nil, nil
	}

	if r, err := handleType("type:", resourceCompletionList); r != nil || err != nil {
		return r, err
	}

	return handleType("Function:", functionCompletionList)
}

// Return the list of currently loaded packages.
func (s *server) pkgCompletionList(pad bool) *protocol.CompletionList {
	s.schemas.m.Lock()
	defer s.schemas.m.Unlock()
	return &protocol.CompletionList{
		Items: util.MapOver(util.MapValues(s.schemas.cache), func(p *schema.Package) protocol.CompletionItem {
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

// Return the completion list of modules for a given package.
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

// The completion list of resources for a package. Only packages whose module is
// prefixed by modHint are returned. If modHint is empty, then only resources in
// the `index` namespace are returned.
func resourceCompletionList(pkg *schema.Package, modHint string) []protocol.CompletionItem {
	return buildCompletionList(protocol.CompletionItemKindClass, func(pkg *schema.Package) []util.Tuple[string, bool] {
		out := make([]util.Tuple[string, bool], len(pkg.Resources))
		for i, f := range pkg.Resources {
			out[i].A = f.Token
			out[i].B = f.DeprecationMessage != ""
		}
		return out
	})(pkg, modHint)
}

func functionCompletionList(pkg *schema.Package, modHint string) []protocol.CompletionItem {
	return buildCompletionList(protocol.CompletionItemKindFunction, func(pkg *schema.Package) []util.Tuple[string, bool] {
		out := make([]util.Tuple[string, bool], len(pkg.Functions))
		for i, f := range pkg.Functions {
			out[i].A = f.Token
			out[i].B = f.DeprecationMessage != ""
		}
		return out
	})(pkg, modHint)
}

func buildCompletionList(kind protocol.CompletionItemKind, f func(pkg *schema.Package) []util.Tuple[string, bool]) func(pkg *schema.Package, modHint string) []protocol.CompletionItem {
	return func(pkg *schema.Package, modHint string) []protocol.CompletionItem {
		out := []protocol.CompletionItem{}
		for _, r := range f(pkg) {
			token := r.A
			depreciated := r.B
			mod := pkg.TokenToModule(token)
			parts := strings.Split(token, ":")
			name := parts[len(parts)-1]

			// We want to use modHint as a prefix (a weak fuzzy filter), but only if
			// modHint is "". When modHint is "", we interpret it as looking at the
			// "index" module, so we need exact matches.
			if (strings.HasPrefix(mod, modHint) && modHint != "") || mod == modHint {
				out = append(out, protocol.CompletionItem{
					Deprecated:       depreciated,
					FilterText:       token,
					InsertText:       name,
					InsertTextFormat: protocol.InsertTextFormatPlainText,
					InsertTextMode:   protocol.InsertTextModeAsIs,
					Kind:             kind,
					Label:            name,
				})
			}
		}
		return out
	}
}

// completeKey returns the completion list for a key at `params.Position`.
func (s *server) completeKey(c lsp.Client, doc *document, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	parents, ok, err := parentKeys(doc.text, params.Position)
	parents = util.ReverseList(parents)
	if err != nil || !ok {
		c.LogDebugf("Could not find enclosing (ok=%t) (err=%v)", ok, err)
		return nil, err
	}
	// The properties key in a resource
	if len(parents) == 3 &&
		strings.ToLower(parents[0].B) == "properties" &&
		strings.ToLower(parents[2].B) == "resources" {
		sibs, ok, err := siblingKeys(doc.text, parents[0].A)
		if !ok || err != nil {
			return nil, err
		}
		typKey, ok := sibs["type"]
		if !ok {
			c.LogDebugf("Completing resource properties but could not find type")
			return nil, nil
		}
		typ, err := doc.text.Line(int(typKey.Line))
		if err != nil {
			return nil, err
		}
		if els := strings.Split(typ, ":"); len(els) == 2 {
			typ = strings.TrimSpace(els[1])
		} else {
			c.LogDebugf("Completing resource properties: found malformed type line: %q", typ)
			return nil, nil
		}
		existingProperties, err := subsidiaryKeys(doc.text, parents[0].A)
		if err != nil {
			return nil, err
		}
		return s.completeProperties(c, typ, util.MapKeys(existingProperties))
	}
	return nil, nil
}

// completeProperties computes the set of properties that don't already exist
// for the represented by `token`, then returns a completion list for those
// remaining properties.
func (s *server) completeProperties(c lsp.Client, token string, existing []string,
) (*protocol.CompletionList, error) {
	resource, err := s.schemas.ResolveResource(c, token)
	if err != nil || resource == nil {
		return nil, err
	}
	// TODO: see completeProperties doc comment
	return nil, nil
}
