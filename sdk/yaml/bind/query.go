// Copyright 2022, Pulumi Corporation.  All rights reserved.

package bind

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi-lsp/sdk/util"
)

// Return a list of all resources whose token matches `tk`.
func (d *Decl) GetResources(tk, version string) ([]Resource, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	// First we load the token, so we can get the alias list
	pkgName, err := pkgNameFromToken(tk)
	if err != nil {
		return nil, fmt.Errorf("cannot get resources: %w", err)
	}
	pkg, ok := d.loadedPackages[pkgKey{pkgName, version}]
	// We didn't have access to that package
	if !ok {
		return nil, fmt.Errorf(
			"package '%s' is not loaded for query '%s', loaded packages are %s",
			pkgName, tk, util.MapKeys(d.loadedPackages))
	}
	if pkg.p == nil {
		return nil, fmt.Errorf("failed to load pkg '%s'", pkgName)
	}
	r, ok := pkg.ResolveResource(tk)
	names := []string{tk}
	if ok {
		for _, a := range r.Aliases {
			names = append(names, *a.Type)
		}
	}
	var rs []Resource
	for _, v := range d.variables {
		if r, ok := v.definition.(*Resource); ok && util.SliceContains(names, r.token) {
			rs = append(rs, *r)
		}
	}
	return rs, nil
}

// Get all invokes used in the program.
func (d *Decl) Invokes() []Invoke {
	return util.DerefList(util.MapKeys(d.invokes))
}

// Retrieve the diagnostic list for the Decl.
func (b *Decl) Diags() hcl.Diagnostics {
	if b == nil {
		return nil
	}
	return b.diags
}

// Return a list of all variable references bound in the Decl.
func (d *Decl) References() []Reference {
	refs := []Reference{}
	for _, v := range d.variables {
		refs = append(refs, v.uses...)
	}
	return refs
}

// Return a list of all variables bound in the Decl.
func (d *Decl) Variables() map[string]*Variable {
	return d.variables
}
