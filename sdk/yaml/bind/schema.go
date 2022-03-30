package bind

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Loads schemas as necessary from the loader to attach to resources and invokes.
// The schemas are cached internally to make searching faster.
// New diagnostics are appended to the internal diag list.
func (d *Decl) LoadSchema(loader schema.Loader) {
	for invoke := range d.invokes {
		typeLoc := invoke.defined.Token.Syntax().Syntax().Range()
		pkgName := d.loadPackage(invoke.token, loader,
			invoke.defined.Token.Syntax().Syntax().Range())
		if pkgName != "" {
			pkg := d.loadedPackages[pkgName]
			if pkg.diag != nil {
				// We don't have a valid package, so give an appropriate warning to the user and move on
				d.diags = d.diags.Extend(pkg.diag(typeLoc))
				continue
			}
			if !pkg.isValid() {
				continue
			}
			if f, ok := pkg.invokes[invoke.token]; ok {
				// There is something wrong with this definition
				if f.diag != nil {
					d.diags = d.diags.Extend(f.diag(typeLoc))
				}
				invoke.definition = f.Function
			} else {
				d.diags = append(d.diags, missingTokenDiag(pkgName, invoke.token, typeLoc))
			}
		}
	}

	for _, v := range d.variables {
		if resource, ok := v.definition.(Resource); ok {
			typeLoc := resource.defined.Value.Type.Syntax().Syntax().Range()
			pkgName := d.loadPackage(resource.token, loader, typeLoc)
			if pkgName != "" {
				pkg := d.loadedPackages[pkgName]
				if pkg.diag != nil {
					// We don't have a valid package, so give an appropriate warning to the user and move on
					d.diags = d.diags.Extend(pkg.diag(typeLoc))
					continue
				}
				if !pkg.isValid() {
					continue
				}
				if f, ok := pkg.resources[resource.token]; ok {
					// There is something wrong with this definition
					if f.diag != nil {
						d.diags = d.diags.Extend(f.diag(typeLoc))
					}
					resource.definition = f.Resource
				} else {
					d.diags = append(d.diags, missingTokenDiag(pkgName, resource.token, typeLoc))
				}
			}
		}
	}
}

// Load a package into the cache if necessary. errRange is the location that
// motivated loading the package (a type token in a invoke or a resource).
func (d *Decl) loadPackage(tk string, loader schema.Loader, errRange *hcl.Range) string {
	pkgName, err := pkgNameFromToken(tk)
	if err != nil {
		d.diags = append(d.diags, unparsableTokenDiag(tk, errRange, err))
		return ""
	}

	pkg, ok := d.loadedPackages[pkgName]
	if !ok {
		p, err := loader.LoadPackage(pkgName, nil)
		if err != nil {
			var d diagsFromLocation
			pkg = pkgCache{
				diag: d.wrap(func(r *hcl.Range) *hcl.Diagnostic {
					return failedToLoadPackageDiag(pkgName, r, err)
				}),
			}
		} else {
			pkg = newPkgCache(p)
		}
		d.loadedPackages[pkgName] = pkg
	}
	return pkgName
}

func pkgNameFromToken(tk string) (string, error) {
	components := strings.Split(tk, ":")
	if len(components) == 2 {
		if tk == "pulumi:providers" {
			return "", fmt.Errorf("package missing from provider type")
		}
		return components[0], nil
	}
	if len(components) == 3 {
		if strings.HasPrefix(tk, "pulumi:providers:") {
			return components[2], nil
		}
		return components[0], nil
	}
	return "", fmt.Errorf("wrong number of components")
}

type diagsFromLocation func(*hcl.Range) hcl.Diagnostics

// Wraps a possibly nil diagsFromLocation in another diagnostic.
func (d *diagsFromLocation) wrap(f func(*hcl.Range) *hcl.Diagnostic) diagsFromLocation {
	return func(r *hcl.Range) hcl.Diagnostics {
		var diags hcl.Diagnostics
		if d != nil {
			diags = append(diags, (*d)(r)...)
		}
		return append(diags, f(r))
	}
}

// Maintain a cached map from tokens to specs.
type pkgCache struct {
	p         *schema.Package
	resources map[string]ResourceSpec
	invokes   map[string]FunctionSpec

	// A package level warning, applied to every resource loaded with the
	// package.
	diag diagsFromLocation
}

func (p pkgCache) isValid() bool {
	if p.p != nil {
		contract.Assert(p.resources != nil)
		contract.Assert(p.invokes != nil)
	}
	return p.p != nil
}

type ResourceSpec struct {
	*schema.Resource

	diag diagsFromLocation
}

type FunctionSpec struct {
	*schema.Function

	diag diagsFromLocation
}

func newPkgCache(p *schema.Package) pkgCache {
	resources := map[string]ResourceSpec{}
	invokes := map[string]FunctionSpec{}
	var d diagsFromLocation
	insertResource := func(k string, v *schema.Resource) {
		f, alreadyUsed := resources[v.Token]
		if alreadyUsed {
			f.diag = d.wrap(func(r *hcl.Range) *hcl.Diagnostic {
				return multipleResourcesDiag(v.Token, r)
			})
		} else {
			if v.DeprecationMessage != "" {
				d = d.wrap(func(r *hcl.Range) *hcl.Diagnostic {
					return depreciatedDiag(v.Token, v.DeprecationMessage, r)
				})
			}
			resources[v.Token] = ResourceSpec{v, d}
		}

	}
	for _, invoke := range p.Functions {
		_, ok := invokes[invoke.Token]
		contract.Assertf(!ok, "Duplicate invokes found for token %s", invoke.Token)
		var dep diagsFromLocation
		if invoke.DeprecationMessage != "" {
			dep = dep.wrap(func(r *hcl.Range) *hcl.Diagnostic {
				return depreciatedDiag(invoke.Token, invoke.DeprecationMessage, r)
			})
		}
		invokes[invoke.Token] = FunctionSpec{invoke, dep}
	}
	for _, r := range p.Resources {
		insertResource(r.Token, r)
		for _, alias := range r.Aliases {
			if tk := alias.Type; tk != nil {
				insertResource(*tk, r)
			}
		}
	}
	return pkgCache{p, resources, invokes, nil}
}
