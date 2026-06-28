// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hclast "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/ast"
	"github.com/zclconf/go-cty/cty"
)

// providerRef is the resolved provider for a Terraform resource/data type: which
// Pulumi package to load its schema from, whether it is a native Pulumi provider
// (declared `source = "pulumi/..."`) or a bridged Terraform provider, the
// declared source, and the pinned version when known.
type providerRef struct {
	localName string          // the provider's local name (the type prefix)
	source    string          // declared source address, e.g. "hashicorp/random" ("" if none)
	pkg       string          // the Pulumi package / plugin name to resolve against
	isPulumi  bool            // source begins with "pulumi/"
	version   *semver.Version // pinned version from the lockfile or constraint, if known
}

// providerFor determines the provider for a resource/data type, honoring the
// program's `required_providers` source (and the lockfile, if present) rather
// than blindly trusting the type's leading segment.
func providerFor(config *hclast.Config, lock map[string]*semver.Version, tfType string) providerRef {
	local := tfType
	if before, _, ok := strings.Cut(tfType, "_"); ok && before != "" {
		local = before
	}
	ref := providerRef{localName: local, pkg: local}

	if config != nil && config.Terraform != nil {
		if rp, ok := config.Terraform.RequiredProviders[local]; ok && rp != nil {
			ref.source = rp.Source
			ref.isPulumi = rp.IsPulumi()
			ref.pkg = packageFromSource(rp.Source, local)
			ref.version = pinnedVersion(lock, ref.pkg, rp.Version)
		}
	}
	return ref
}

// packageFromSource maps a Terraform source address to the Pulumi package name:
// the last path segment. "pulumi/aws" → "aws", "hashicorp/random" → "random",
// "integrations/github" → "github". An empty source falls back to the local name.
func packageFromSource(source, local string) string {
	if source == "" {
		return local
	}
	parts := strings.Split(source, "/")
	return parts[len(parts)-1]
}

// pinnedVersion prefers an exact version from the lockfile (keyed by package
// name); otherwise it tries to parse the required_providers constraint as an
// exact version (only succeeds for pins like "5.4.0", not ranges like "~> 5.0").
func pinnedVersion(lock map[string]*semver.Version, pkg, constraint string) *semver.Version {
	if v, ok := lock[pkg]; ok {
		return v
	}
	if constraint != "" {
		if v, err := semver.ParseTolerant(constraint); err == nil {
			return &v
		}
	}
	return nil
}

// readLockfile parses a sibling .terraform.lock.hcl, returning exact pinned
// versions keyed by Pulumi package name (the source's last segment). Returns nil
// when there is no lockfile or it cannot be parsed.
func readLockfile(dir string) map[string]*semver.Version {
	data, err := os.ReadFile(filepath.Join(dir, ".terraform.lock.hcl"))
	if err != nil {
		return nil
	}
	file, diags := hclsyntax.ParseConfig(data, ".terraform.lock.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}

	out := map[string]*semver.Version{}
	for _, block := range body.Blocks {
		// provider "registry.terraform.io/hashicorp/random" { version = "3.6.2" ... }
		if block.Type != "provider" || len(block.Labels) == 0 {
			continue
		}
		attr, ok := block.Body.Attributes["version"]
		if !ok {
			continue
		}
		val, vdiags := attr.Expr.Value(nil)
		if vdiags.HasErrors() || val.Type() != cty.String {
			continue
		}
		v, err := semver.ParseTolerant(val.AsString())
		if err != nil {
			continue
		}
		pkg := lastSegment(block.Labels[0]) // ".../hashicorp/random" → "random"
		out[pkg] = &v
	}
	return out
}

func lastSegment(s string) string {
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}

// dirForURI returns the on-disk directory containing a document URI, used to
// locate a sibling lockfile. Returns "" when the URI is not a file path.
func dirForURI(uri string) string {
	const filePrefix = "file://"
	if !strings.HasPrefix(uri, filePrefix) {
		return ""
	}
	return filepath.Dir(strings.TrimPrefix(uri, filePrefix))
}
