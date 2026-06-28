// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"testing"

	hclparser "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.lsp.dev/protocol"
)

const sampleProgram = `terraform {
  required_providers {
    tls = {
      source = "pulumi/tls"
    }
  }
}

resource "tls_private_key" "key" {
  algorithm = "RSA"
}
`

func TestObjectAtPoint(t *testing.T) {
	t.Parallel()
	config, diags := hclparser.NewParser().ParseSource("Main.tf", []byte(sampleProgram))
	require.False(t, diags.HasErrors(), "%v", diags)

	r := config.Resources["tls_private_key.key"]
	require.NotNil(t, r)

	// A point in the middle of the type label.
	typePos := protocol.Position{
		Line:      uint32(r.TypeRange.Start.Line - 1),
		Character: uint32(r.TypeRange.Start.Column + 2), // a few chars into the label
	}
	onType := objectAtPoint(config, typePos)
	require.NotNil(t, onType)
	assert.True(t, onType.onTypeLabel)
	assert.Equal(t, "tls_private_key", onType.resource.Type)

	// A point on the line below the header — inside the block body.
	bodyPos := protocol.Position{
		Line:      uint32(r.DeclRange.Start.Line), // header line + 1 (0-indexed)
		Character: 4,
	}
	inBody := objectAtPoint(config, bodyPos)
	require.NotNil(t, inBody)
	assert.False(t, inBody.onTypeLabel)
	assert.Equal(t, "tls_private_key", inBody.resource.Type)

	// A position well outside any block.
	assert.Nil(t, objectAtPoint(config, protocol.Position{Line: 1000, Character: 0}))
}

func TestProviderFor(t *testing.T) {
	t.Parallel()
	// sampleProgram declares `tls = { source = "pulumi/tls" }`.
	config, _ := hclparser.NewParser().ParseSource("Main.tf", []byte(sampleProgram))
	ref := providerFor(config, nil, "tls_private_key")
	assert.Equal(t, "tls", ref.pkg)
	assert.True(t, ref.isPulumi)
	assert.Equal(t, "pulumi/tls", ref.source)

	// A bridged source maps to the package name (last segment), not the local name.
	bridged, _ := hclparser.NewParser().ParseSource("b.tf", []byte(`terraform {
  required_providers {
    github = { source = "integrations/github" }
  }
}
resource "github_repository" "r" {}`))
	gh := providerFor(bridged, nil, "github_repository")
	assert.Equal(t, "github", gh.pkg)
	assert.False(t, gh.isPulumi)
	assert.Equal(t, "integrations/github", gh.source)

	// With no required_providers block, fall back to the type's leading segment.
	bare, _ := hclparser.NewParser().ParseSource("c.tf", []byte(`resource "aws_s3_bucket" "b" {}`))
	assert.Equal(t, "aws", providerFor(bare, nil, "aws_s3_bucket").pkg)
}

func TestPackageFromSource(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "aws", packageFromSource("pulumi/aws", "aws"))
	assert.Equal(t, "random", packageFromSource("hashicorp/random", "random"))
	assert.Equal(t, "github", packageFromSource("integrations/github", "gh"))
	assert.Equal(t, "local", packageFromSource("", "local"))
}

func TestSnakeCase(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ecdsa_curve", snakeCase("ecdsaCurve"))
	assert.Equal(t, "rsa_bits", snakeCase("rsaBits"))
	assert.Equal(t, "algorithm", snakeCase("algorithm"))
}
