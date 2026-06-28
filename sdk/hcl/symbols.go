// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"sort"

	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
)

// documentSymbol produces an outline of the program's top-level declarations:
// resources, data sources, variables, locals, outputs, providers and modules.
func (s *server) documentSymbol(client lsp.Client, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	config, ok := s.configFor(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	var symbols []protocol.DocumentSymbol

	add := func(name, detail string, kind protocol.SymbolKind, full hcl.Range, sel hcl.Range) {
		fullRange := convertRange(&full)
		symbols = append(symbols, protocol.DocumentSymbol{
			Name:           name,
			Detail:         detail,
			Kind:           kind,
			Range:          fullRange,
			SelectionRange: convertRange(&sel),
		})
	}

	for key, r := range config.Resources {
		add(key, "resource", protocol.SymbolKindClass, resourceRange(r), r.TypeRange)
	}
	for key, r := range config.DataSources {
		add(key, "data source", protocol.SymbolKindClass, resourceRange(r), r.TypeRange)
	}
	for name, v := range config.Variables {
		add(name, "variable", protocol.SymbolKindVariable, v.DeclRange, v.DeclRange)
	}
	for name, l := range config.Locals {
		add(name, "local", protocol.SymbolKindConstant, l.DeclRange, l.DeclRange)
	}
	for name, o := range config.Outputs {
		add(name, "output", protocol.SymbolKindProperty, o.DeclRange, o.DeclRange)
	}
	for key, p := range config.Providers {
		add(key, "provider", protocol.SymbolKindNamespace, p.DeclRange, p.DeclRange)
	}
	for name, m := range config.Modules {
		add(name, "module", protocol.SymbolKindModule, m.DeclRange, m.DeclRange)
	}

	// Stable, source-order output so the outline does not jitter (the AST stores
	// most declarations in maps, whose iteration order is random).
	sort.Slice(symbols, func(i, j int) bool {
		a, b := symbols[i].Range.Start, symbols[j].Range.Start
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Character < b.Character
	})

	out := make([]interface{}, len(symbols))
	for i := range symbols {
		out[i] = symbols[i]
	}
	return out, nil
}
