// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"
)

// spanTagRe matches the per-language <span pulumi-lang-*="..."> wrappers that
// Pulumi schema docs embed; we strip the tags and keep the inner text.
var spanTagRe = regexp.MustCompile(`</?span[^>]*>`)

// cleanDoc removes schema-specific HTML wrappers so doc comments render cleanly
// as Markdown hover content.
func cleanDoc(s string) string {
	return spanTagRe.ReplaceAllString(s, "")
}

// convertRange maps an hcl.Range onto an LSP protocol.Range. HCL positions are
// 1-indexed for both line and column; LSP positions are 0-indexed.
func convertRange(r *hcl.Range) protocol.Range {
	if r == nil {
		return protocol.Range{}
	}
	return protocol.Range{
		Start: convertPosition(r.Start),
		End:   convertPosition(r.End),
	}
}

func convertPosition(p hcl.Pos) protocol.Position {
	var zero hcl.Pos
	if p == zero {
		return protocol.Position{}
	}
	return protocol.Position{
		Line:      uint32(max(p.Line-1, 0)),
		Character: uint32(max(p.Column-1, 0)),
	}
}

func convertSeverity(s hcl.DiagnosticSeverity) protocol.DiagnosticSeverity {
	switch s {
	case hcl.DiagError:
		return protocol.DiagnosticSeverityError
	case hcl.DiagWarning:
		return protocol.DiagnosticSeverityWarning
	default:
		return protocol.DiagnosticSeverityInformation
	}
}

// diagMessage renders an hcl.Diagnostic into a single human-readable string,
// appending the detail beneath the summary when present.
func diagMessage(d *hcl.Diagnostic) string {
	if d.Detail == "" {
		return d.Summary
	}
	return d.Summary + "\n" + d.Detail
}

// posBefore reports whether a comes strictly before b.
func posBefore(a, b protocol.Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Character < b.Character
}

// posInRange reports whether an LSP position falls within an HCL range
// (inclusive of both ends).
func posInRange(r hcl.Range, pos protocol.Position) bool {
	start := convertPosition(r.Start)
	end := convertPosition(r.End)
	return !posBefore(pos, start) && !posBefore(end, pos)
}

// snakeCase converts a Pulumi camelCase property name (e.g. "bucketPrefix") to
// the snake_case form used in Terraform HCL (e.g. "bucket_prefix"). This is a
// display/convention approximation; exact field mapping for bridged Terraform
// providers comes from the bridge BodyMapping (M2b).
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
