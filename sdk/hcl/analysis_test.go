// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	hclparser "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.lsp.dev/protocol"
)

// parseToDiagnostics mirrors what the parse stage does, returning the LSP
// diagnostics that would be published for a given document.
func parseToDiagnostics(t *testing.T, src string) []protocol.Diagnostic {
	t.Helper()
	config, diags := hclparser.NewParser().ParseSource("Main.tf", []byte(src))
	// A partial config is expected even when there are diagnostics.
	_ = config
	return toLSPDiagnostics(diags)
}

func TestValidProgramHasNoErrorDiagnostics(t *testing.T) {
	t.Parallel()
	const src = `
resource "aws_s3_bucket" "my_bucket" {
  bucket = "my-unique-bucket-name"
  tags = {
    Environment = "dev"
  }
}

output "bucket_arn" {
  value = aws_s3_bucket.my_bucket.arn
}
`
	diags := parseToDiagnostics(t, src)
	for _, d := range diags {
		assert.NotEqualf(t, protocol.DiagnosticSeverityError, d.Severity,
			"valid program should not produce an error diagnostic: %s", d.Message)
	}
}

func TestMalformedProgramProducesErrorWithRange(t *testing.T) {
	t.Parallel()
	// Unterminated resource block -> syntactic error.
	const src = `resource "aws_s3_bucket" "my_bucket" {
  bucket = "oops"
`
	diags := parseToDiagnostics(t, src)
	require.NotEmpty(t, diags, "malformed program should produce diagnostics")

	var sawError bool
	for _, d := range diags {
		if d.Severity == protocol.DiagnosticSeverityError {
			sawError = true
			// The range must point somewhere real (not the zero value), and
			// must be 0-indexed (HCL is 1-indexed; we convert).
			assert.NotEqual(t, protocol.Range{}, d.Range,
				"error diagnostic should carry a source range")
			assert.NotEmpty(t, d.Message, "error diagnostic should have a message")
			assert.Equal(t, "pulumi-hcl", d.Source)
		}
	}
	assert.True(t, sawError, "expected at least one error-severity diagnostic")
}

func TestConvertPositionIsZeroIndexed(t *testing.T) {
	t.Parallel()
	// HCL line/column are 1-indexed; LSP expects 0-indexed.
	got := convertPosition(hcl.Pos{Line: 3, Column: 5, Byte: 0})
	assert.Equal(t, uint32(2), got.Line)
	assert.Equal(t, uint32(4), got.Character)
}
