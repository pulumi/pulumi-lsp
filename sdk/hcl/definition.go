// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hclast "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/ast"
	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
)

// definition implements go-to-definition for references such as `var.x`,
// `local.x`, `module.x`, `data.type.name`, and resource references
// `type.name[.attr]`, jumping to the referenced declaration.
func (s *server) definition(client lsp.Client, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	config, ok := s.configFor(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	for _, t := range traversalsIn(config) {
		if !posInRange(t.SourceRange(), params.Position) {
			continue
		}
		target, ok := definitionTarget(config, traversalNames(t))
		if !ok {
			return nil, nil
		}
		return []protocol.Location{{
			URI:   params.TextDocument.URI,
			Range: convertRange(&target),
		}}, nil
	}
	return nil, nil
}

// definitionTarget maps a traversal's leading name segments to the source range
// of the declaration they reference.
func definitionTarget(config *hclast.Config, names []string) (hcl.Range, bool) {
	if len(names) == 0 {
		return hcl.Range{}, false
	}
	switch names[0] {
	case "var":
		if len(names) >= 2 {
			if v, ok := config.Variables[names[1]]; ok {
				return v.DeclRange, true
			}
		}
	case "local":
		if len(names) >= 2 {
			if l, ok := config.Locals[names[1]]; ok {
				return l.DeclRange, true
			}
		}
	case "module":
		if len(names) >= 2 {
			if m, ok := config.Modules[names[1]]; ok {
				return m.DeclRange, true
			}
		}
	case "data":
		if len(names) >= 3 {
			if r, ok := config.DataSources[names[1]+"."+names[2]]; ok {
				return r.DeclRange, true
			}
		}
	default:
		// A bare `type.name` is a managed-resource reference.
		if len(names) >= 2 {
			if r, ok := config.Resources[names[0]+"."+names[1]]; ok {
				return r.DeclRange, true
			}
		}
	}
	return hcl.Range{}, false
}

// traversalNames extracts the leading identifier segments of a traversal
// (stopping at the first non-name traverser such as an index).
func traversalNames(t hcl.Traversal) []string {
	var names []string
	for _, p := range t {
		switch tp := p.(type) {
		case hcl.TraverseRoot:
			names = append(names, tp.Name)
		case hcl.TraverseAttr:
			names = append(names, tp.Name)
		default:
			return names
		}
	}
	return names
}

// traversalsIn collects every scope traversal expression in the program, so the
// one under the cursor can be located.
func traversalsIn(config *hclast.Config) []hcl.Traversal {
	var c traversalCollector
	for _, f := range config.Files {
		if body, ok := f.Body.(*hclsyntax.Body); ok {
			_ = hclsyntax.Walk(body, &c)
		}
	}
	return c.traversals
}

type traversalCollector struct {
	traversals []hcl.Traversal
}

func (c *traversalCollector) Enter(node hclsyntax.Node) hcl.Diagnostics {
	if e, ok := node.(*hclsyntax.ScopeTraversalExpr); ok {
		c.traversals = append(c.traversals, e.Traversal)
	}
	return nil
}

func (c *traversalCollector) Exit(node hclsyntax.Node) hcl.Diagnostics { return nil }
