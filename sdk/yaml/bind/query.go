package bind

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
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
			pkgName, tk, mapKeys(d.loadedPackages))
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
		if r, ok := v.definition.(*Resource); ok && sliceContains(names, r.token) {
			rs = append(rs, *r)
		}
	}
	return rs, nil
}

// Get all invokes used in the program.
func (d *Decl) Invokes() []Invoke {
	return derefList(mapKeys(d.invokes))
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

func mapKeys[K comparable, V any](m map[K]V) []K {
	arr := make([]K, len(m))
	i := 0
	for k := range m {
		arr[i] = k
		i++
	}
	return arr
}

func derefList[T any](l []*T) []T {
	ls := make([]T, len(l))
	for i := range l {
		ls[i] = *l[i]
	}
	return ls
}
