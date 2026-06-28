// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"bytes"
	"fmt"

	"github.com/pulumi-labs/pulumi-hcl/pkg/hcl/bridge"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"go.lsp.dev/protocol"
)

// describeResource renders a Pulumi resource schema as Markdown hover content.
// Argument names are shown using their Terraform field names (from the bridge
// body mapping when available) to match how they are written in HCL.
func describeResource(r *schema.Resource, bm *bridge.BodyMapping) protocol.MarkupContent {
	names := fieldNames(bm)
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "# Resource: `%s`\n", r.Token)
	if r.Comment != "" {
		fmt.Fprintf(b, "\n%s\n", cleanDoc(r.Comment))
	}
	if r.DeprecationMessage != "" {
		fmt.Fprintf(b, "\n## Deprecated\n%s\n", r.DeprecationMessage)
	}
	if len(r.InputProperties) > 0 {
		fmt.Fprint(b, "\n## Arguments\n")
		for _, input := range r.InputProperties {
			writeProperty(b, input, names)
		}
	}
	if len(r.Properties) > 0 {
		fmt.Fprint(b, "\n## Attributes\n")
		for _, output := range r.Properties {
			writeProperty(b, output, names)
		}
	}
	return protocol.MarkupContent{Kind: protocol.Markdown, Value: b.String()}
}

// describeFunction renders a Pulumi function (data source) schema as Markdown.
func describeFunction(f *schema.Function, bm *bridge.BodyMapping) protocol.MarkupContent {
	names := fieldNames(bm)
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "# Data Source: `%s`\n", f.Token)
	if f.Comment != "" {
		fmt.Fprintf(b, "\n%s\n", cleanDoc(f.Comment))
	}
	if f.DeprecationMessage != "" {
		fmt.Fprintf(b, "\n## Deprecated\n%s\n", f.DeprecationMessage)
	}
	if f.Inputs != nil && len(f.Inputs.Properties) > 0 {
		fmt.Fprint(b, "\n## Arguments\n")
		for _, input := range f.Inputs.Properties {
			writeProperty(b, input, names)
		}
	}
	if f.Outputs != nil && len(f.Outputs.Properties) > 0 {
		fmt.Fprint(b, "\n## Attributes\n")
		for _, output := range f.Outputs.Properties {
			writeProperty(b, output, names)
		}
	}
	return protocol.MarkupContent{Kind: protocol.Markdown, Value: b.String()}
}

// describeProperty renders hover content for a single argument/attribute,
// labelled with its Terraform field name.
func describeProperty(prop *schema.Property, tfName string) protocol.MarkupContent {
	b := &bytes.Buffer{}
	required := ""
	if prop.IsRequired() {
		required = " _(required)_"
	}
	fmt.Fprintf(b, "**`%s`** `%s`%s\n", tfName, codegen.UnwrapType(prop.Type).String(), required)
	if prop.DeprecationMessage != "" {
		fmt.Fprintf(b, "\n_Deprecated: %s_\n", firstLine(cleanDoc(prop.DeprecationMessage)))
	}
	if prop.Comment != "" {
		fmt.Fprintf(b, "\n%s\n", cleanDoc(prop.Comment))
	}
	return protocol.MarkupContent{Kind: protocol.Markdown, Value: b.String()}
}

// findProperty returns the schema property whose Terraform field name matches
// tfName, or nil if none does.
func findProperty(props []*schema.Property, names map[string]string, tfName string) *schema.Property {
	for _, p := range props {
		if tfFieldName(names, p.Name) == tfName {
			return p
		}
	}
	return nil
}

func writeProperty(b *bytes.Buffer, prop *schema.Property, names map[string]string) {
	required := ""
	if prop.IsRequired() {
		required = " _(required)_"
	}
	fmt.Fprintf(b, "- **`%s`** `%s`%s", tfFieldName(names, prop.Name), codegen.UnwrapType(prop.Type).String(), required)
	if prop.Comment != "" {
		fmt.Fprintf(b, " — %s", firstLine(cleanDoc(prop.Comment)))
	}
	fmt.Fprint(b, "\n")
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
