// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// diskCachingMapper persists Terraform-bridge mappings to disk, keyed by
// provider name and installed plugin version, so the expensive mapping
// computation is performed once per machine and survives LSP restarts. (The
// upstream caching mapper is in-memory and per-process only.) The mapping for a
// given provider+version is deterministic, so the version is part of the key
// and an upgrade naturally invalidates the entry.
type diskCachingMapper struct {
	inner convert.Mapper
	dir   string
}

// newDiskCachingMapper wraps a mapper with on-disk persistence. If no user cache
// directory is available the inner mapper is returned unwrapped.
func newDiskCachingMapper(inner convert.Mapper) convert.Mapper {
	dir, err := os.UserCacheDir()
	if err != nil {
		return inner
	}
	return &diskCachingMapper{
		inner: inner,
		dir:   filepath.Join(dir, "pulumi-lsp", "hcl-mappings"),
	}
}

var unsafeKeyChars = regexp.MustCompile(`[^a-zA-Z0-9._@-]`)

func (m *diskCachingMapper) GetMapping(
	ctx context.Context, provider string, hint *convert.MapperPackageHint,
) ([]byte, error) {
	key := provider
	if v := installedVersion(ctx, provider); v != "" {
		key = provider + "@" + v
	}
	path := filepath.Join(m.dir, unsafeKeyChars.ReplaceAllString(key, "_")+".json")

	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		return data, nil
	}

	data, err := m.inner.GetMapping(ctx, provider, hint)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		writeFileAtomic(m.dir, path, data) // best-effort; a miss just recomputes
	}
	return data, nil
}

// writeFileAtomic writes data to path via a unique temp file and a rename, so a
// concurrent reader never observes a partially written file and two concurrent
// writers cannot interleave into a corrupt one. Best-effort: any error is
// ignored (a cache miss simply recomputes next time).
func writeFileAtomic(dir, path string, data []byte) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
	}
}

// installedVersion returns the version of the installed resource plugin for the
// given provider, or "" when it cannot be determined.
func installedVersion(ctx context.Context, provider string) string {
	plugins, err := pluginstorage.Instance.GetPlugins(ctx)
	if err != nil {
		return ""
	}
	for _, p := range plugins {
		if p.Kind == apitype.ResourcePlugin && p.Name == provider && p.Version != nil {
			return p.Version.String()
		}
	}
	return ""
}
