package bind

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Loads schemas as necessary from the loader to attach to resources and invokes.
// The schemas are cached internally to make searching faster.
// New diagnostics are appended to the internal diag list.
func (d *Decl) LoadSchema(loader schema.Loader) {
	d.lock.Lock()
	defer d.lock.Unlock()
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
			if f, ok := pkg.ResolveFunction(invoke.token); ok {
				// There is something wrong with this definition
				if f.diag != nil {
					d.diags = d.diags.Extend(f.diag(typeLoc))
				}
				invoke.definition = f.Function
				if ret := invoke.defined.Return; ret != nil {
					if out := f.Function.Outputs; out != nil {
						var valid bool
						existing := map[string]struct{}{}
						for _, prop := range out.Properties {
							if prop.Name == ret.Value {
								valid = true
								break
							}
							existing[prop.Name] = struct{}{}
						}
						if !valid {
							d.diags = append(d.diags, propertyDoesNotExistDiag(ret.Value, out, mapKeys(existing), ret.Syntax().Syntax().Range()))
						}
					}
				}
			} else {
				d.diags = append(d.diags, missingTokenDiag(pkgName, invoke.token, typeLoc))
			}
		}
	}

	for _, v := range d.variables {
		if resource, ok := v.definition.(*Resource); ok {
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
				if f, ok := pkg.ResolveResource(resource.token); ok {
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

	d.checkSchemaPropertyAccesses()
}

func (d Decl) typeExpr(e ast.Expr) schema.Type {
	switch e := e.(type) {
	// Primitive types: nothing to bind
	case *ast.NullExpr:
		return nil
	case *ast.BooleanExpr:
		return schema.BoolType
	case *ast.NumberExpr:
		return schema.NumberType
	case *ast.StringExpr, *ast.InterpolateExpr:
		return schema.StringType

	case *ast.SymbolExpr:
		var tag string
		if t, ok := e.Property.Accessors[0].(*ast.PropertyName); ok {
			tag = t.Name
		}
		if v, ok := d.variables[tag]; tag != "" && ok {
			t := v.definition.ResolveType(&d)
			if len(e.Property.Accessors) == 0 {
				return t
			}
			for _, r := range v.uses {
				if r.location == e.Syntax().Syntax().Range() {
					types, _ := r.access.Typed(t)
					return types[len(types)-1]
				}
			}
		}
		return nil

	// Container types:
	case *ast.ListExpr:
		t := schema.AnyType
		if len(e.Elements) != 0 {
			t = d.typeExpr(e.Elements[0])
		}
		return &schema.ArrayType{ElementType: t}

	case *ast.ObjectExpr:
		// TODO handle object types
		return nil

	case *ast.InvokeExpr:
		t := e.Token
		if t == nil {
			return nil
		}
		for invoke := range d.invokes {
			if invoke.token == t.Value {
				if invoke.definition != nil {
					outputs := invoke.definition.Outputs
					if e.Return != nil {
						p, ok := outputs.Property(e.Return.Value)
						if ok {
							return p.Type
						}
						return nil
					}
					return outputs
				}
				break
			}
		}
		return nil

	case *ast.AssetArchiveExpr, *ast.FileArchiveExpr, *ast.RemoteArchiveExpr:
		return schema.ArchiveType
	case *ast.FileAssetExpr, *ast.RemoteAssetExpr, *ast.StringAssetExpr:
		return schema.AssetType

	// Stack reference
	case *ast.StackReferenceExpr:
		return nil

	// Functions
	case *ast.JoinExpr:
		return schema.StringType
	case *ast.SelectExpr:
		el := d.typeExpr(e.Values)
		if el == nil {
			return nil
		}
		if e, ok := el.(*schema.ArrayType); ok {
			return e.ElementType
		}
		return nil
	case *ast.SplitExpr:
		return schema.StringType
	case *ast.ToBase64Expr:
		return schema.StringType
	case *ast.ToJSONExpr:
		return schema.StringType
	default:
		return nil
	}
}

// We have resolved variables and invokes to their schema equivalents. We can
// now resolve property access, posting diagnostics as necessary.
func (d *Decl) checkSchemaPropertyAccesses() {
	for _, v := range d.variables {
		for _, use := range v.uses {
			d.checkSchemaPropertyAccess(v.definition, use)
		}
	}
}

// Check that a property access is valid against the schema. If no schema can be
// found, say nothing.
func (d *Decl) checkSchemaPropertyAccess(def Definition, use Reference) {
	// No point checking if there is no access
	if len(use.access) == 0 {
		return
	}
	switch def := def.(type) {
	case *Resource:
		if def.definition == nil {
			return
		}
		d.verifyPropertyAccess(use.access, &schema.ResourceType{
			Token:    def.definition.Token,
			Resource: def.definition,
		})
	case ResolvableType:
		typ := def.ResolveType(d)
		if typ == nil {
			return
		}
		d.verifyPropertyAccess(use.access, typ)
	default:
		contract.Failf("Unknown definition type: %[1]T: %[1]v", def)
	}
}

func (d *Decl) verifyPropertyAccess(expr PropertyAccessorList, typ schema.Type) {
	if len(expr) == 0 {
		return
	}
	_, diag := expr.Typed(typ)
	if diag != nil {
		d.diags = append(d.diags, diag)
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
			pkg = pkgCache{
				diag: NewDiagsFromLocation(func(r *hcl.Range) *hcl.Diagnostic {
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

func NewDiagsFromLocation(f func(*hcl.Range) *hcl.Diagnostic) diagsFromLocation {
	return func(r *hcl.Range) hcl.Diagnostics {
		return hcl.Diagnostics{f(r)}
	}
}

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

func (p *pkgCache) ResolveResource(token string) (ResourceSpec, bool) {
	tk, err := yaml.NewResourcePackage(p.p).ResolveResource(token)
	if err != nil {
		return ResourceSpec{}, false
	}
	r, ok := p.resources[string(tk)]
	return r, ok
}

func (p *pkgCache) ResolveFunction(token string) (FunctionSpec, bool) {
	tk, err := yaml.NewResourcePackage(p.p).ResolveFunction(token)
	if err != nil {
		return FunctionSpec{}, false
	}
	f, ok := p.invokes[string(tk)]
	return f, ok
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
	insertResource := func(k string, v *schema.Resource) {
		f, alreadyUsed := resources[v.Token]
		if alreadyUsed {
			f.diag = NewDiagsFromLocation(func(r *hcl.Range) *hcl.Diagnostic {
				return multipleResourcesDiag(v.Token, r)
			})
		} else {
			r := ResourceSpec{v, nil}
			if v.DeprecationMessage != "" {
				r.diag = NewDiagsFromLocation(func(r *hcl.Range) *hcl.Diagnostic {
					return depreciatedDiag(v.Token, v.DeprecationMessage, r)
				})
			}
			resources[v.Token] = r
		}

	}
	for _, invoke := range p.Functions {
		_, ok := invokes[invoke.Token]
		contract.Assertf(!ok, "Duplicate invokes found for token %s", invoke.Token)
		spec := FunctionSpec{invoke, nil}
		if invoke.DeprecationMessage != "" {
			spec.diag = NewDiagsFromLocation(func(r *hcl.Range) *hcl.Diagnostic {
				return depreciatedDiag(invoke.Token, invoke.DeprecationMessage, r)
			})
		}
		invokes[invoke.Token] = spec
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
