// Copyright 2026, Pulumi Corporation.  All rights reserved.

// Package pluginhost constructs a Pulumi plugin context backed by the standard
// plugin host, which boots installed provider plugins to load package schemas.
// It is shared by the LSP entrypoint and by tests that need a real schema
// loader.
package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// NewContext builds a plugin context whose host loads installed provider plugins
// to resolve package schemas. The returned context's Host is owned by the
// caller and must be closed separately (see Close).
func NewContext() (*plugin.Context, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Host logging is discarded: the LSP communicates over stdio, so writing to
	// stdout/stderr would risk corrupting the JSON-RPC stream.
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})

	// The package registry is only consulted to resolve registry-style package
	// references, which the LSP does not do. Schema loading of installed plugins
	// goes through the loader server and does not need it, so fail lazily.
	reg := registry.NewOnDemandRegistry(func() (registry.Registry, error) {
		return nil, fmt.Errorf("package registry resolution is unavailable in the LSP")
	})

	host, err := pkghost.New(context.Background(), sink, sink, nil,
		pkgWorkspace.EnsureLanguageInstalled,
		schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
		packageworkspace.NewResolverServer(reg))
	if err != nil {
		return nil, err
	}

	return plugin.NewContextWithHost(context.Background(), sink, sink, host, pwd, pwd, nil)
}

// Close releases a context created by NewContext along with its host (which the
// context does not own). Both are closed even if the first close fails.
func Close(pctx *plugin.Context) error {
	host := pctx.Host
	return errors.Join(pctx.Close(), host.Close())
}
