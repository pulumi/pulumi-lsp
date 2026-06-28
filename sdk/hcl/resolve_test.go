// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"context"
	"strings"
	"testing"

	hclparser "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/parser"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-lsp/sdk/pluginhost"
)

// TestResolveAndDescribe is an integration test: it resolves a real provider
// schema through the installed-plugin loader and renders hover/completion. It
// requires the `tls` provider plugin to be installed
// (`pulumi plugin install resource tls`); it skips otherwise so it does not
// fail in environments without the plugin.
func TestResolveAndDescribe(t *testing.T) {
	pctx, err := pluginhost.NewContext()
	require.NoError(t, err)
	defer func() { _ = pluginhost.Close(pctx) }()

	s := &server{
		loader:     schema.NewPluginLoader(pctx),
		infoSource: newProviderInfoSource(pctx),
	}
	config, diags := hclparser.NewParser().ParseSource("Main.tf", []byte(sampleProgram))
	require.False(t, diags.HasErrors(), "%v", diags)

	res, err := s.resolveResource(context.Background(), providerFor(config, nil, "tls_private_key"), "tls_private_key")
	if err != nil || res == nil {
		t.Skipf("tls provider not installed (resolve failed: %v); "+
			"run `pulumi plugin install resource tls` to enable this test", err)
	}

	// Resolution landed on the expected Pulumi token.
	require.Equal(t, "tls:index/privateKey:PrivateKey", res.Token)

	// Hover markdown carries the token and Terraform argument names.
	bm := s.resourceBodyMapping(context.Background(), providerFor(config, nil, "tls_private_key"), "tls_private_key")
	hover := describeResource(res, bm).Value
	require.Contains(t, hover, "tls:index/privateKey:PrivateKey")
	require.Contains(t, hover, "ecdsa_curve")
	require.Contains(t, hover, "rsa_bits")

	// Completion offers the input properties with Terraform field names.
	names := fieldNames(bm)
	var labels []string
	for _, p := range res.InputProperties {
		labels = append(labels, tfFieldName(names, p.Name))
	}
	require.Contains(t, labels, "algorithm")
	require.Contains(t, labels, "ecdsa_curve")
	require.Contains(t, labels, "rsa_bits")
	require.False(t, strings.Contains(strings.Join(labels, ","), "ecdsaCurve"),
		"property names should be Terraform-cased for HCL")
}

// TestPropertyHover checks hover over an attribute name inside a block: it
// should describe that property by its Terraform field name. Requires `tls`.
func TestPropertyHover(t *testing.T) {
	pctx, err := pluginhost.NewContext()
	require.NoError(t, err)
	defer func() { _ = pluginhost.Close(pctx) }()

	s := &server{loader: schema.NewPluginLoader(pctx), infoSource: newProviderInfoSource(pctx)}
	config, _ := hclparser.NewParser().ParseSource("Main.tf", []byte(sampleProgram))

	res, err := s.resolveResource(context.Background(), providerFor(config, nil, "tls_private_key"), "tls_private_key")
	if err != nil || res == nil {
		t.Skip("tls provider not installed")
	}
	names := fieldNames(s.resourceBodyMapping(context.Background(), providerFor(config, nil, "tls_private_key"), "tls_private_key"))

	// `ecdsa_curve` is the Terraform name for the Pulumi property `ecdsaCurve`.
	prop := findProperty(res.InputProperties, names, "ecdsa_curve")
	require.NotNil(t, prop, "ecdsa_curve should map to a property")
	md := describeProperty(prop, "ecdsa_curve").Value
	require.Contains(t, md, "ecdsa_curve")
	require.NotContains(t, md, "ecdsaCurve")

	// A non-existent attribute resolves to no property.
	require.Nil(t, findProperty(res.InputProperties, names, "not_a_real_field"))
}

// TestBridgeResolvesNativeProvider checks the M2b win: a native (non-bridged)
// provider whose schema does NOT follow Terraform naming — and therefore fails
// the convention-only path — resolves via the in-process bridge mapper.
// Requires `pulumi plugin install resource random`; skips otherwise.
func TestBridgeResolvesNativeProvider(t *testing.T) {
	pctx, err := pluginhost.NewContext()
	require.NoError(t, err)
	defer func() { _ = pluginhost.Close(pctx) }()

	s := &server{
		loader:     schema.NewPluginLoader(pctx),
		infoSource: newProviderInfoSource(pctx),
	}
	if s.infoSource == nil {
		t.Skip("could not build in-process bridge mapper")
	}

	config, _ := hclparser.NewParser().ParseSource("Main.tf",
		[]byte(`resource "random_pet" "p" {}`))

	res, err := s.resolveResource(context.Background(), providerFor(config, nil, "random_pet"), "random_pet")
	if err != nil || res == nil {
		t.Skipf("random provider not installed (resolve failed: %v); "+
			"run `pulumi plugin install resource random` to enable this test", err)
	}
	require.Equal(t, "random:index/randomPet:RandomPet", res.Token)
}
