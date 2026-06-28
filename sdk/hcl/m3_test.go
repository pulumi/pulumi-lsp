// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"context"
	"testing"

	"github.com/hashicorp/hcl/v2"
	hclast "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/ast"
	hclparser "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/step"
	"github.com/pulumi/pulumi-lsp/sdk/util"
)

func noClient() lsp.Client { return lsp.Client{} }

func lspDocument(uri protocol.DocumentURI, text string) lsp.Document {
	return lsp.NewDocument(protocol.TextDocumentItem{
		URI:        uri,
		LanguageID: "terraform",
		Version:    1,
		Text:       text,
	})
}

// stubAnalysis builds a pipeline whose parse stage is already complete, so the
// read-only handlers can be exercised without a running client.
func stubAnalysis(config *hclast.Config) *documentAnalysisPipeline {
	ctx, cancel := context.WithCancel(context.Background())
	d := &documentAnalysisPipeline{ctx: ctx, cancel: cancel}
	d.parsed = step.New(ctx, func() (util.Tuple[*hclast.Config, hcl.Diagnostics], bool) {
		return util.Tuple[*hclast.Config, hcl.Diagnostics]{A: config}, true
	})
	d.parsed.GetResult() // block until the (immediate) parse completes
	return d
}

const refProgram = `variable "name" {
  type = string
}

locals {
  greeting = "hello"
}

resource "tls_private_key" "key" {
  algorithm = "RSA"
}

output "fingerprint" {
  value = tls_private_key.key.public_key_fingerprint_md5
}

output "echo" {
  value = var.name
}
`

func parseRef(t *testing.T) (*server, protocol.DocumentURI) {
	t.Helper()
	s := &server{docs: map[protocol.DocumentURI]*document{}}
	uri := protocol.DocumentURI("file:///Main.tf")
	doc := s.setDocument(lspDocument(uri, refProgram))
	config, diags := hclparser.NewParser().ParseSource(uri.Filename(), []byte(refProgram))
	require.False(t, diags.HasErrors(), "%v", diags)
	// Stub the analysis with a completed parse so the handlers can read it.
	doc.analysis = stubAnalysis(config)
	return s, uri
}

func TestDocumentSymbols(t *testing.T) {
	t.Parallel()
	s, uri := parseRef(t)
	syms, err := s.documentSymbol(noClient(), &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	require.NoError(t, err)

	byName := map[string]protocol.DocumentSymbol{}
	for _, sym := range syms {
		ds := sym.(protocol.DocumentSymbol)
		byName[ds.Name] = ds
	}
	require.Contains(t, byName, "name")
	assert.Equal(t, "variable", byName["name"].Detail)
	require.Contains(t, byName, "greeting")
	assert.Equal(t, "local", byName["greeting"].Detail)
	require.Contains(t, byName, "tls_private_key.key")
	assert.Equal(t, "resource", byName["tls_private_key.key"].Detail)
	require.Contains(t, byName, "fingerprint")
	assert.Equal(t, "output", byName["fingerprint"].Detail)
}

func TestDefinition(t *testing.T) {
	t.Parallel()
	s, uri := parseRef(t)
	config, _ := s.configFor(uri)

	// Find the position of the `var.name` reference in the `echo` output.
	var varRef protocol.Position
	for _, tr := range traversalsIn(config) {
		names := traversalNames(tr)
		if len(names) >= 2 && names[0] == "var" && names[1] == "name" {
			r := tr.SourceRange()
			varRef = protocol.Position{Line: uint32(r.Start.Line - 1), Character: uint32(r.Start.Column)}
		}
	}

	locs, err := s.definition(noClient(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     varRef,
		},
	})
	require.NoError(t, err)
	require.Len(t, locs, 1)
	// The definition should point at the `variable "name"` block on line 1.
	assert.Equal(t, uint32(0), locs[0].Range.Start.Line)

	// A resource reference resolves to its block.
	var resRef protocol.Position
	for _, tr := range traversalsIn(config) {
		names := traversalNames(tr)
		if len(names) >= 2 && names[0] == "tls_private_key" && names[1] == "key" {
			r := tr.SourceRange()
			resRef = protocol.Position{Line: uint32(r.Start.Line - 1), Character: uint32(r.Start.Column)}
		}
	}
	locs, err = s.definition(noClient(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     resRef,
		},
	})
	require.NoError(t, err)
	require.Len(t, locs, 1)
	assert.Equal(t, uint32(8), locs[0].Range.Start.Line) // resource block on line 9 (0-indexed 8)
}

func TestCleanDoc(t *testing.T) {
	t.Parallel()
	in := `use <span pulumi-lang-go="PrivateKey" pulumi-lang-python="PrivateKey">tls.PrivateKey</span> instead`
	assert.Equal(t, "use tls.PrivateKey instead", cleanDoc(in))
}
