// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"context"

	"github.com/blang/semver"

	"github.com/pulumi-labs/pulumi-hcl/pkg/hcl/bridge"
	"github.com/pulumi-labs/pulumi-hcl/pkg/hcl/packages"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// newProviderInfoSource builds a Terraform-bridge provider info source entirely
// in-process from the plugin context — no Pulumi engine or gRPC mapper required.
// It boots installed provider plugins on demand to obtain their bridge mappings.
// On failure it returns nil, in which case resolution falls back to the
// convention-based path (which still resolves Terraform-bridged providers).
func newProviderInfoSource(pctx *plugin.Context) bridge.ProviderInfoSource {
	base, err := convert.NewBasePluginMapper(
		pluginstorage.Instance,
		"terraform",
		convert.ProviderFactoryFromHost(pctx.Base(), pctx),
		func(string) *semver.Version { return nil }, // never auto-install plugins
		nil,
	)
	if err != nil {
		return nil
	}
	// Layer caches: in-memory (fastest, per-process) over on-disk persistence
	// (per machine, survives restarts) over the plugin-booting base mapper.
	mapper := convert.NewCachingMapper(newDiskCachingMapper(base))
	return bridge.NewCache(bridge.NewMapperSource(mapper))
}

// newResolver constructs a bridge-aware resolver scoped to a resource type's
// provider, using the Pulumi package name derived from the program's declared
// `source` (honoring `pulumi/...` vs bridged Terraform providers) rather than
// the bare local name. Construction is cheap; the expensive bridge mapping is
// memoized inside the shared provider info source.
func (s *server) newResolver(ref providerRef) *packages.Resolver {
	return packages.NewResolver(s.loader, s.infoSource, nil, []string{ref.pkg})
}

func (s *server) resolveResource(ctx context.Context, ref providerRef, tfType string) (*schema.Resource, error) {
	return s.newResolver(ref).ResolveResource(ctx, tfType)
}

func (s *server) resolveFunction(ctx context.Context, ref providerRef, tfType string) (*schema.Function, error) {
	return s.newResolver(ref).ResolveFunction(ctx, tfType)
}

func (s *server) resourceBodyMapping(ctx context.Context, ref providerRef, tfType string) *bridge.BodyMapping {
	return s.newResolver(ref).ResourceBodyMapping(ctx, tfType)
}

func (s *server) dataSourceBodyMapping(ctx context.Context, ref providerRef, tfType string) *bridge.BodyMapping {
	return s.newResolver(ref).DataSourceBodyMapping(ctx, tfType)
}

// fieldNames builds a Pulumi-property-name → Terraform-field-name lookup from a
// bridge body mapping. Returns nil when no mapping is available. Building this
// once avoids an O(properties × fields) scan when naming every property.
func fieldNames(bm *bridge.BodyMapping) map[string]string {
	if bm == nil {
		return nil
	}
	names := make(map[string]string, len(bm.Fields))
	for _, fm := range bm.Fields {
		names[fm.PulumiName] = fm.TFName
	}
	return names
}

// tfFieldName returns the Terraform field name for a Pulumi property. When the
// bridge mapping has it, that is authoritative; otherwise we approximate from
// the Pulumi camelCase name.
func tfFieldName(names map[string]string, pulumiName string) string {
	if tf, ok := names[pulumiName]; ok {
		return tf
	}
	return snakeCase(pulumiName)
}
