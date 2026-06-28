// Copyright 2026, Pulumi Corporation.  All rights reserved.

package hcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	hclparser "github.com/pulumi-labs/pulumi-hcl/pkg/hcl/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLockfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `provider "registry.terraform.io/hashicorp/random" {
  version     = "3.6.2"
  constraints = ">= 3.0.0"
  hashes      = ["h1:abc"]
}

provider "registry.terraform.io/integrations/github" {
  version = "6.3.0"
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".terraform.lock.hcl"), []byte(content), 0o644))

	lock := readLockfile(dir)
	require.NotNil(t, lock)
	require.NotNil(t, lock["random"])
	assert.Equal(t, "3.6.2", lock["random"].String())
	require.NotNil(t, lock["github"])
	assert.Equal(t, "6.3.0", lock["github"].String())

	// No lockfile present → nil.
	assert.Nil(t, readLockfile(t.TempDir()))
}

func TestProviderForUsesLockedVersion(t *testing.T) {
	t.Parallel()
	config, _ := hclparser.NewParser().ParseSource("Main.tf", []byte(`terraform {
  required_providers {
    random = { source = "hashicorp/random", version = ">= 3.0.0" }
  }
}
resource "random_pet" "p" {}`))

	// Lockfile pins an exact version; the range constraint cannot.
	v := semver.MustParse("3.6.2")
	lock := map[string]*semver.Version{"random": &v}

	ref := providerFor(config, lock, "random_pet")
	assert.Equal(t, "random", ref.pkg)
	require.NotNil(t, ref.version)
	assert.Equal(t, "3.6.2", ref.version.String())

	// Without a lockfile, a range constraint yields no exact pin.
	refNoLock := providerFor(config, nil, "random_pet")
	assert.Nil(t, refNoLock.version)
}

func TestDirForURI(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/a/b", dirForURI("file:///a/b/Main.tf"))
	assert.Equal(t, "", dirForURI("untitled:Untitled-1"))
}
