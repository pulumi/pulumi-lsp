// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hclast "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/ast"
	"go.lsp.dev/protocol"
)

// pointTarget describes the AST node under the editor cursor. For M2a we care
// about resource and data-source blocks: whether the cursor is on the type
// label (drives hover / resource-type completion) or inside the block body
// (drives attribute completion).
type pointTarget struct {
	resource *hclast.Resource
	// onTypeLabel is true when the cursor is over the resource's type label
	// (e.g. the `aws_s3_bucket` in `resource "aws_s3_bucket" "b" {}`).
	onTypeLabel bool
}

// objectAtPoint locates the resource or data-source block containing pos.
// Returns nil when the cursor is not inside a resolvable block.
func objectAtPoint(config *hclast.Config, pos protocol.Position) *pointTarget {
	if config == nil {
		return nil
	}

	// Managed resources and data sources share the ast.Resource shape; data
	// sources carry IsDataSource = true and resolve as functions.
	for _, group := range []map[string]*hclast.Resource{config.Resources, config.DataSources} {
		for _, r := range group {
			if r == nil {
				continue
			}
			if posInRange(r.TypeRange, pos) {
				return &pointTarget{resource: r, onTypeLabel: true}
			}
			if posInRange(resourceRange(r), pos) {
				return &pointTarget{resource: r, onTypeLabel: false}
			}
		}
	}
	return nil
}

// resourceRange returns the full source span of a resource block — from the
// `resource`/`data` keyword through the closing brace. DeclRange alone only
// covers the block header, so the body range is taken from the parsed body.
func resourceRange(r *hclast.Resource) hcl.Range {
	full := r.DeclRange
	if body, ok := r.Config.(*hclsyntax.Body); ok {
		full.End = body.SrcRange.End
	}
	return full
}

// attributeAtPoint returns the name and range of the body attribute whose name
// label is under pos (e.g. `algorithm` in `algorithm = "RSA"`), if any.
func attributeAtPoint(r *hclast.Resource, pos protocol.Position) (string, hcl.Range, bool) {
	body, ok := r.Config.(*hclsyntax.Body)
	if !ok {
		return "", hcl.Range{}, false
	}
	for name, attr := range body.Attributes {
		if posInRange(attr.NameRange, pos) {
			return name, attr.NameRange, true
		}
	}
	return "", hcl.Range{}, false
}
