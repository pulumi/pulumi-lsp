// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-lsp/sdk/pluginhost"
)

func TestResourceTypeLineDetection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line          string
		wantKind      string
		wantPartial   string
		shouldNotMatch bool
	}{
		{`resource "tls_`, "resource", "tls_", false},
		{`  resource "aws_s3`, "resource", "aws_s3", false},
		{`data "aws_ami`, "data", "aws_ami", false},
		{`resource "tls_private_key" "k" {`, "", "", true}, // past the label
		{`  algorithm = "RSA"`, "", "", true},              // inside the body
	}
	for _, c := range cases {
		m := resourceTypeLineRe.FindStringSubmatch(c.line)
		if c.shouldNotMatch {
			assert.Nil(t, m, "line should not match: %q", c.line)
			continue
		}
		require.NotNil(t, m, "line should match: %q", c.line)
		assert.Equal(t, c.wantKind, m[1])
		assert.Equal(t, c.wantPartial, m[2])
	}
}

// TestListTypes is an integration test requiring the `tls` provider plugin.
func TestListTypes(t *testing.T) {
	pctx, err := pluginhost.NewContext()
	require.NoError(t, err)
	defer pluginhost.Close(pctx)

	s := &server{
		loader:     schema.NewPluginLoader(pctx),
		infoSource: newProviderInfoSource(pctx),
	}
	if s.infoSource == nil {
		t.Skip("could not build in-process bridge mapper")
	}

	types := s.listTypes(context.Background(), "tls", false)
	if len(types) == 0 {
		t.Skip("tls provider not installed; run `pulumi plugin install resource tls`")
	}
	assert.Contains(t, types, "tls_private_key")
	assert.Contains(t, types, "tls_cert_request")
	// Sorted.
	assert.True(t, sortStringsIsSorted(types), "results should be sorted")

	// Data sources are a separate catalog.
	dataTypes := s.listTypes(context.Background(), "tls", true)
	assert.Contains(t, dataTypes, "tls_public_key")
}

func sortStringsIsSorted(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}
