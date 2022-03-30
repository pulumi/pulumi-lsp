package bind

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

// Return a list of all resources whose token matches `tk`.
func (d *Decl) GetResources(tk string) ([]Resource, error) {
	// First we load the token, so we can get the alias list
	pkgName, err := pkgNameFromToken(tk)
	if err != nil {
		return nil, fmt.Errorf("Cannot get resources: %w", err)
	}
	pkg, ok := d.loadedPackages[pkgName]
	// We didn't have access to that package
	if !ok || pkg.p == nil {
		return nil, fmt.Errorf("Package '%s' is not loaded for query '%s'", pkgName, tk)
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
		if r, ok := v.definition.(Resource); ok && sliceContains(names, r.token) {
			rs = append(rs, r)
		}
	}
	return rs, nil
}

func (b *Decl) Diags() hcl.Diagnostics {
	if b == nil {
		return nil
	}
	return b.diags
}

func sliceContains[T comparable](slice []T, el T) bool {
	for _, t := range slice {
		if t == el {
			return true
		}
	}
	return false
}
