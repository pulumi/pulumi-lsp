package bind

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"

	"github.com/iwahbe/pulumi-lsp/sdk/util"
)

// Return a list of all resources whose token matches `tk`.
func (d *Decl) GetResources(tk string) ([]Resource, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	// First we load the token, so we can get the alias list
	pkgName, err := pkgNameFromToken(tk)
	if err != nil {
		return nil, fmt.Errorf("Cannot get resources: %w", err)
	}
	pkg, ok := d.loadedPackages[pkgName]
	// We didn't have access to that package
	if !ok {
		return nil, fmt.Errorf(
			"Package '%s' is not loaded for query '%s', loaded packages are %s",
			pkgName, tk, util.MapKeys(d.loadedPackages))
	}
	if pkg.p == nil {
		return nil, fmt.Errorf("Failed to load pkg '%s'", pkgName)
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

func (b *Decl) Diags() hcl.Diagnostics {
	if b == nil {
		return nil
	}
	return b.diags
}

func (d *Decl) References() []Reference {
	refs := []Reference{}
	for _, v := range d.variables {
		refs = append(refs, v.uses...)
	}
	return refs
}

func (d *Decl) Variables() map[string]*Variable {
	return d.variables
}
