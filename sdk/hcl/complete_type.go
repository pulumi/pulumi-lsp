// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"context"
	"regexp"
	"sort"
	"strings"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
)

// resourceTypeLineRe matches a partially-typed resource/data block header up to
// the cursor: the block keyword, the opening quote, and the partial type name.
// It is line/text based because a half-typed block does not parse, so the AST
// cannot be consulted here.
var resourceTypeLineRe = regexp.MustCompile(`(?:^|\s)(resource|data)\s+"([a-zA-Z0-9_]*)$`)

// completeResourceType offers resource (or data source) type-name suggestions
// while the cursor is inside the type label, e.g. `resource "aws_s3_b…`.
func (s *server) completeResourceType(
	client lsp.Client, doc *document, params *protocol.CompletionParams,
) (*protocol.CompletionList, error) {
	if s.infoSource == nil {
		return nil, nil
	}
	line, err := doc.text.Line(int(params.Position.Line))
	if err != nil {
		return nil, nil
	}
	col := int(params.Position.Character)
	if col > len(line) {
		col = len(line)
	}
	m := resourceTypeLineRe.FindStringSubmatch(line[:col])
	if m == nil {
		return nil, nil
	}
	isData := m[1] == "data"
	partial := m[2]

	// Resolve the Pulumi package from the program's declared `source` (honoring
	// pulumi/… vs bridged providers), falling back to the type's leading segment.
	config, _ := s.configFor(params.TextDocument.URI)
	lock := readLockfile(dirForURI(string(params.TextDocument.URI)))
	provider := providerFor(config, lock, partial).pkg
	if provider == "" {
		return nil, nil
	}

	types := s.listTypes(client.Context(), provider, isData)
	if len(types) == 0 {
		// Most often this means the provider plugin is not installed. Surface a
		// breadcrumb in the output channel, once per provider per session.
		if _, seen := s.warnedProviders.LoadOrStore(provider, true); !seen {
			client.LogInfof("pulumi-hcl: no types found for provider %q — is the plugin installed? "+
				"Try `pulumi plugin install resource %s`", provider, provider)
		}
		return nil, nil
	}

	// Replace exactly the partial type already typed so selection is clean.
	editRange := protocol.Range{
		Start: protocol.Position{Line: params.Position.Line, Character: uint32(col - len(partial))},
		End:   protocol.Position{Line: params.Position.Line, Character: uint32(col)},
	}

	items := make([]protocol.CompletionItem, 0, len(types))
	for _, t := range types {
		if !strings.HasPrefix(t, partial) {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:    t,
			Kind:     protocol.CompletionItemKindClass,
			TextEdit: &protocol.TextEdit{Range: editRange, NewText: t},
		})
	}
	return &protocol.CompletionList{Items: items}, nil
}

// listTypes returns the Terraform resource (or data source) type names for a
// provider, sourced from the in-process bridge mapping.
func (s *server) listTypes(ctx context.Context, provider string, isData bool) []string {
	info, err := s.infoSource.GetProviderInfo(ctx, provider, nil)
	if err != nil || info == nil || info.P == nil {
		return nil
	}
	var m shim.ResourceMap
	if isData {
		m = info.P.DataSourcesMap()
	} else {
		m = info.P.ResourcesMap()
	}
	if m == nil {
		return nil
	}
	var out []string
	m.Range(func(key string, _ shim.Resource) bool {
		out = append(out, key)
		return true
	})
	sort.Strings(out)
	return out
}
