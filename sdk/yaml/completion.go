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
		return s.propertyListCompletion(util.ResourceProperties(t.Resource), filterPrefix)
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

	handleType := func(prefix string, resolve func(schema.PackageReference, string) []protocol.CompletionItem) (*protocol.CompletionList, error) {
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

				// "pulumi" is a special package with a single module: "providers"
				if parts[0] == "pulumi" {
					return &protocol.CompletionList{
						Items: []protocol.CompletionItem{{
							CommitCharacters: []string{":"},
							FilterText:       "pulumi:providers",
							InsertText:       "providers:",
							InsertTextFormat: protocol.InsertTextFormatPlainText,
							InsertTextMode:   protocol.InsertTextModeAsIs,
							Kind:             protocol.CompletionItemKindModule,
							Label:            "providers",
						}},
					}, nil
				}
				pkg, err := s.schemas.Loader(client).LoadPackageReference(parts[0], nil)
				if err != nil {
					return nil, err
				}
				mods := moduleCompletionList(pkg)
				client.LogDebugf("Found %d modules for %s", len(mods), pkg.Name())
				return &protocol.CompletionList{
					Items: append(mods, resolve(pkg, "")...),
				}, nil
			case 3:
				if parts[0] == "pulumi" {
					if parts[1] == "providers" {
						s.schemas.m.Lock()
						defer s.schemas.m.Unlock()
						return &protocol.CompletionList{
							IsIncomplete: false,
							Items: util.MapOver(util.MapKeys(s.schemas.cache), func(t util.Tuple[string, string]) protocol.CompletionItem {
								mod := t.A
								return protocol.CompletionItem{
									FilterText:       "pulumi:providers:" + mod,
									InsertText:       mod,
									InsertTextFormat: protocol.InsertTextFormatPlainText,
									InsertTextMode:   protocol.InsertTextModeAsIs,
									Kind:             protocol.CompletionItemKindModule,
									Label:            mod,
								}
							}),
						}, nil
					}
					// There are no valid completions for this token
					return nil, nil
				}
				pkg, err := s.schemas.Loader(client).LoadPackageReference(parts[0], nil)
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
	insertText := func(name string) string {
		name = name + ":"
		if pad {
			name = " " + name
		}
		return name
	}
	return &protocol.CompletionList{
		Items: append(util.MapOver(util.MapValues(s.schemas.cache), func(p schema.PackageReference) protocol.CompletionItem {
			return protocol.CompletionItem{
				CommitCharacters: []string{":"},
				// TODO: Add back after https://github.com/pulumi/pulumi/pull/9800 merges
				// Documentation:    p.Description(),
				FilterText:       p.Name(),
				InsertText:       insertText(p.Name()),
				InsertTextFormat: protocol.InsertTextFormatPlainText,
				InsertTextMode:   protocol.InsertTextModeAsIs,
				Kind:             protocol.CompletionItemKindModule,
				Label:            p.Name(),
			}
		}), protocol.CompletionItem{
			CommitCharacters: []string{":"},
			FilterText:       "pulumi",
			InsertText:       insertText("pulumi"),
			InsertTextFormat: protocol.InsertTextFormatPlainText,
			InsertTextMode:   protocol.InsertTextModeAsIs,
			Kind:             protocol.CompletionItemKindModule,
			Label:            "pulumi",
		}),
	}
}

// Return the completion list of modules for a given package.
func moduleCompletionList(pkg schema.PackageReference) []protocol.CompletionItem {
	m := map[string]struct{}{}
	for it := pkg.Resources().Range(); it.Next(); {
		s := pkg.TokenToModule(it.Token())
		m[s] = struct{}{}
	}
	out := make([]protocol.CompletionItem, 0, len(m))
	for mod := range m {
		full := pkg.Name() + ":" + mod
		out = append(out, protocol.CompletionItem{
			CommitCharacters: []string{":"},
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
func resourceCompletionList(pkg schema.PackageReference, modHint string) []protocol.CompletionItem {
	return buildCompletionList(protocol.CompletionItemKindClass, func(pkg schema.PackageReference) []util.Tuple[string, bool] {
		out := []util.Tuple[string, bool]{}
		for it := pkg.Resources().Range(); it.Next(); {
			tup := util.Tuple[string, bool]{}
			tup.A = it.Token()
			f, err := it.Resource()
			if err != nil {
				tup.B = f.DeprecationMessage != ""
			}
			out = append(out, tup)
		}
		return out
	})(pkg, modHint)
}

func functionCompletionList(pkg schema.PackageReference, modHint string) []protocol.CompletionItem {
	return buildCompletionList(protocol.CompletionItemKindFunction, func(pkg schema.PackageReference) []util.Tuple[string, bool] {
		out := []util.Tuple[string, bool]{}
		for it := pkg.Functions().Range(); it.Next(); {
			tup := util.Tuple[string, bool]{}
			tup.A = it.Token()
			f, err := it.Function()
			if err != nil {
				tup.B = f.DeprecationMessage != ""
			}
			out = append(out, tup)
		}

		return out
	})(pkg, modHint)
}

func buildCompletionList(
	kind protocol.CompletionItemKind, f func(pkg schema.PackageReference) []util.Tuple[string, bool],
) func(pkg schema.PackageReference, modHint string) []protocol.CompletionItem {
	return func(pkg schema.PackageReference, modHint string) []protocol.CompletionItem {
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

// Generate a set of unique completions. `existing` is the list of existing keys
// while `keys` has the form {A: label, B: itemFn} where `itemFn(c)` will set up
// `c` as the completion item with the label A.
func uniqueCompletions(existing []string,
	keys []util.Tuple[string, func(*protocol.CompletionItem)]) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	e := util.NewSet(util.MapOver(existing, strings.ToLower)...)
	for _, key := range keys {
		if !e.Has(key.A) {
			i := protocol.CompletionItem{
				Label: key.A,
			}
			key.B(&i)
			items = append(items, i)
		}
	}
	return items
}

// completeKey returns the completion list for a key at `params.Position`.
func (s *server) completeKey(c lsp.Client, doc *document, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	parents, indentation, ok, err := parentKeys(doc.text, params.Position)
	parents = util.ReverseList(parents)
	if err != nil {
		c.LogDebugf("Could not find enclosing (ok=%t) (err=%v)", ok, err)
		return nil, err
	}
	postFix := postFix{indentation}
	switch {
	case !ok: // We are at the top level
		return completeTopLevelKeys(doc)

	// Completing for the ResourceOptions decl
	case len(parents) == 3 &&
		strings.ToLower(parents[0].B) == "options" &&
		strings.ToLower(parents[2].B) == "resources":
		return completeResourceOptionsKeys(doc, parents[0].A, postFix)

	// Completing for the Resource decl
	case len(parents) == 2 && strings.ToLower(parents[1].B) == "resources":
		return completeResourceKeys(doc, parents[0].A, postFix)

	// The properties key in a resource
	case len(parents) == 3 &&
		strings.ToLower(parents[0].B) == "properties" &&
		strings.ToLower(parents[2].B) == "resources":
		return completeResourcePropertyKeys(c, doc, parents[0].A, s, postFix)

	// The Arguments key in a function
	case len(parents) > 3 &&
		strings.ToLower(parents[0].B) == "arguments" &&
		strings.ToLower(parents[1].B) == "fn::invoke":
		return completeFunctionArgumentKeys(c, doc, parents[1].A, parents[0].A, s, postFix, len(parents))

	default:
		return nil, nil
	}
}

func completeTopLevelKeys(doc *document) (*protocol.CompletionList, error) {
	sibs, err := topLevelKeys(doc.text)
	if err != nil {
		return nil, err
	}
	setDetails := func(detail string) func(*protocol.CompletionItem) {
		return func(c *protocol.CompletionItem) {
			c.Detail = detail
			// NOTE: This assumes that 2 spaces are used for indentation
			c.InsertText = c.Label + ":\n  "
		}
	}
	return &protocol.CompletionList{
		Items: uniqueCompletions(util.MapOver(util.MapKeys(sibs), func(s string) string {
			s = strings.ToLower(s)
			s = strings.TrimSpace(s)
			parts := strings.Split(s, ":")
			if len(parts) > 0 {
				s = parts[0]
			}
			return s
		}), []util.Tuple[string, func(*protocol.CompletionItem)]{
			{A: "configuration", B: setDetails("Configuration values used in Pulumi YAML")},
			{A: "resources", B: setDetails("A map of of Pulumi Resources")},
			{A: "outputs", B: setDetails("A map of outputs")},
			{A: "variables", B: setDetails("A map of variable names to their values")},
		}),
	}, nil
}

func completeResourceOptionsKeys(doc *document, keyPos protocol.Position, post postFix) (*protocol.CompletionList, error) {
	sibs, err := childKeys(doc.text, keyPos)
	if err != nil {
		return nil, err
	}
	setDetails := func(detail, doc string, post func(int) string) func(*protocol.CompletionItem) {
		return func(c *protocol.CompletionItem) {
			c.Detail = detail
			// We are indenting to the forth level
			// resources:
			//   name:
			//     options:
			//       what we are suggestion
			c.InsertText = c.Label + post(4)
			c.Documentation = doc
			c.InsertTextMode = protocol.InsertTextModeAsIs
		}
	}

	return &protocol.CompletionList{
		Items: uniqueCompletions(util.MapOver(util.MapKeys(sibs), func(s string) string {
			s = strings.ToLower(s)
			s = strings.TrimSpace(s)
			parts := strings.Split(s, ":")
			if len(parts) > 0 {
				s = parts[0]
			}
			return s
		}), []util.Tuple[string, func(*protocol.CompletionItem)]{
			{A: "additionalSecretOutputs",
				B: setDetails("List<String>",
					"AdditionalSecretOutputs specifies properties that must be encrypted as secrets",
					post.intoList)},
			{A: "aliases",
				B: setDetails("List<String>",
					"Aliases specifies names that this resource used to be have so that renaming or refactoring doesn’t replace it",
					post.intoList)},
			{A: "customTimeouts",
				B: setDetails("CustomTimeout",
					"CustomTimeouts overrides the default retry/timeout behavior for resource provisioning",
					post.intoObject)},
			{A: "deleteBeforeReplace",
				B: setDetails("Boolean",
					"DeleteBeforeReplace overrides the default create-before-delete behavior when replacing",
					post.sameLine)},
			{A: "dependsOn",
				B: setDetails("List<Expression>",
					"DependsOn makes this resource explicitly depend on another resource, by name, so that it won't be created before the "+
						"dependent finishes being created (and the reverse for destruction). Normally, Pulumi automatically tracks implicit"+
						" dependencies through inputs/outputs, but this can be used when dependencies aren't captured purely from input/output edges.",
					post.intoList)},
			{A: "ignoreChanges",
				B: setDetails("List<String>",
					"IgnoreChanges declares that changes to certain properties should be ignored during diffing",
					post.intoList)},
			{A: "import",
				B: setDetails("String",
					"Import adopts an existing resource from your cloud account under the control of Pulumi",
					post.sameLine)},
			{A: "parent",
				B: setDetails("Resource",
					"Parent specifies a parent for the resource",
					post.sameLine)},
			{A: "protect",
				B: setDetails("Boolean",
					"Protect prevents accidental deletion of a resource",
					post.sameLine)},
			{A: "provider",
				B: setDetails("Provider Resource",
					"Provider specifies an explicitly configured provider, instead of using the default global provider",
					post.sameLine)},
			{A: "providers",
				B: setDetails("Map<Provider Resource>",
					"Map of providers for a resource and its children.",
					post.intoObject)},
			{A: "version",
				B: setDetails("String",
					"Version specifies a provider plugin version that should be used when operating on a resource",
					post.sameLine)},
		}),
	}, nil
}

func completeResourceKeys(doc *document, keyPos protocol.Position, postFix postFix) (*protocol.CompletionList, error) {
	sibs, err := childKeys(doc.text, keyPos)
	if err != nil {
		return nil, err
	}

	items := []protocol.CompletionItem{}
	existing := map[string]string{}
	for _, s := range util.MapKeys(sibs) {
		s = strings.ToLower(s)
		s = strings.TrimSpace(s)
		parts := strings.Split(s, ":")
		if len(parts) > 0 {
			s = parts[0]
		}
		existing[s] = strings.Join(parts[1:], ":")
	}

	addItem := func(label, detail string, post func(int) string) {
		if _, ok := existing[label]; ok {
			return
		}
		items = append(items, protocol.CompletionItem{
			InsertText:     label + post(3),
			Label:          label,
			Detail:         detail,
			InsertTextMode: protocol.InsertTextModeAsIs,
		})
	}
	isProvider := func(s string) bool {
		s = strings.TrimSpace(s)
		parts := strings.Split(s, ":")
		if len(parts) != 3 {
			return false
		}
		return parts[0] == "pulumi" && parts[1] == "providers"
	}
	// If we don't have a type, it could be a provider, so suggest.
	// If we do have a type, suggest only if it is a provider
	if typ, ok := existing["type"]; !ok || isProvider(typ) {
		addItem("defaultProvider", "If this provider should be the default for its package", postFix.sameLine)
	}
	addItem("properties", "A map of resource properties."+
		" See https://www.pulumi.com/docs/intro/concepts/resources/ for details.", postFix.intoObject)
	addItem("type", "The Pulumi type token for this resource.", postFix.sameLine)
	addItem("options", "A map of resource options."+
		" See https://www.pulumi.com/docs/intro/concepts/resources/options/ for details.", postFix.intoObject)

	return &protocol.CompletionList{Items: items}, nil
}

func completeResourcePropertyKeys(
	c lsp.Client, doc *document, keyPos protocol.Position, s *server, postFix postFix,
) (*protocol.CompletionList, error) {
	sibs, ok, err := siblingKeys(doc.text, keyPos)
	if !ok || err != nil {
		return nil, err
	}
	typKey, ok := sibs["type"]
	if !ok {
		c.LogDebugf("Completing resource properties but could not find type")
		return nil, nil
	}

	typ := getTokenAtLine(doc.text, int(typKey.Line))
	if typ == "" {
		c.LogDebugf("Completing resource properties: found malformed type on line: %q", typKey.Line)
		return nil, nil
	}
	existingProperties, err := childKeys(doc.text, keyPos)
	if err != nil {
		return nil, err
	}
	resource, err := s.schemas.ResolveResource(c, typ)
	if err != nil || resource == nil {
		return nil, err
	}

	return s.completeProperties(c, resource.InputProperties, util.MapKeys(existingProperties), postFix, 4)
}

func completeFunctionArgumentKeys(
	c lsp.Client, doc *document, invokePos, argumentsPos protocol.Position, s *server, postFix postFix, indentLevel int,
) (*protocol.CompletionList, error) {
	keys, err := childKeys(doc.text, invokePos)
	if err != nil {
		return nil, err
	}
	fnKey, ok := keys["function"]
	if !ok {
		return nil, nil
	}
	typ := getTokenAtLine(doc.text, int(fnKey.Line))
	if typ == "" {
		return nil, nil
	}
	existingProperties, err := childKeys(doc.text, argumentsPos)
	if err != nil {
		return nil, err
	}
	fn, err := s.schemas.ResolveFunction(c, typ)
	if err != nil || fn == nil || fn.Inputs == nil {
		return nil, err
	}

	return s.completeProperties(c, fn.Inputs.Properties, util.MapKeys(existingProperties), postFix, indentLevel)
}

func getTokenAtLine(text lsp.Document, line int) string {
	typ, err := text.Line(line)
	if err != nil {
		return ""
	}
	if els := strings.SplitN(typ, ":", 2); len(els) == 2 {
		typ = strings.TrimSpace(els[1])
	} else {
		return ""
	}
	return typ
}

// completeProperties computes the set of properties that don't already exist
// for the represented by `token`, then returns a completion list for those
// remaining properties.
func (s *server) completeProperties(
	c lsp.Client, inputs []*schema.Property, existing []string, postFix postFix, indentLevel int,
) (*protocol.CompletionList, error) {
	props := make([]*schema.Property, len(inputs))
	es := util.NewSet(util.MapOver(existing, strings.ToLower)...)
	for _, p := range inputs {
		if es.Has(strings.ToLower(p.Name)) {
			continue
		}
		contract.Assertf(p != nil, "nil properties are not allowed")
		props = append(props, p)
	}
	completions, err := s.propertyListCompletion(props, "")
	if err != nil {
		return nil, err
	}
	for i, p := range completions.Items {
		f := postFix.sameLine
		switch codegen.UnwrapType(props[i].Type).(type) {
		case *schema.ArrayType:
			f = postFix.intoList
		case *schema.MapType, *schema.ObjectType:
			f = postFix.intoObject
		}
		p.InsertText = p.Label + f(indentLevel)
		p.InsertTextMode = protocol.InsertTextModeAsIs
		completions.Items[i] = p
	}
	return completions, nil
}

type postFix struct {
	indentation int
}

func (p postFix) sameLine(ignored int) string {
	return ": "
}

func (p postFix) intoObject(indentationLevel int) string {
	return ":\n" + strings.Repeat(" ", p.indentation*indentationLevel)
}

func (p postFix) intoList(indentationLevel int) string {
	return p.intoObject(indentationLevel) + "- "
}
