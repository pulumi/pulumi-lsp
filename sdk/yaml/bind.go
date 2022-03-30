package yaml

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type BoundDecl struct {
	variables map[string]*Variable

	// The output namespace
	outputs map[string]ast.PropertyMapEntry

	// The set of all invokes.
	invokes map[*Invoke]struct{}

	diags hcl.Diagnostics
}

type Variable struct {
	// definition is either a Resource, *ast.ConfigParamDecl
	definition interface{}
	uses       []Reference
}

type Reference struct {
	location *hcl.Range
	access   []ast.PropertyAccessor
}

func (b *BoundDecl) NewRefernce(variable string, loc *hcl.Range, accessor []ast.PropertyAccessor) {
	v, ok := b.variables[variable]
	ref := Reference{location: loc, access: accessor}
	if ok {
		v.uses = append(v.uses, ref)
	} else {
		b.variables[variable] = &Variable{uses: []Reference{ref}}
	}
}

func (v *Variable) DefinitionRange() *hcl.Range {
	switch d := v.definition.(type) {
	case ast.ConfigMapEntry:
		return d.Key.Syntax().Syntax().Range()
	case ast.VariablesMapEntry:
		return d.Key.Syntax().Syntax().Range()
	case Resource:
		return d.defined.Key.Syntax().Syntax().Range()
	default:
		panic(fmt.Errorf("Unexpected variable type %T", d))
	}
}

type Resource struct {
	token      string
	defined    *ast.ResourcesMapEntry
	definition *schema.Resource
}

type Invoke struct {
	token      string
	defined    *ast.InvokeExpr
	definition *schema.Function
}

func NewBoundDecl(decl *ast.TemplateDecl) (*BoundDecl, error) {
	bound := &BoundDecl{map[string]*Variable{}, map[string]ast.PropertyMapEntry{}, map[*Invoke]struct{}{}, hcl.Diagnostics{}}

	for _, c := range decl.Configuration.Entries {
		other, alreadyReferenced := bound.variables[c.Key.Value]
		if alreadyReferenced && other.definition != nil {
			bound.diags = bound.diags.Append(
				duplicateSource(c.Key.Value,
					c.Key.Syntax().Syntax().Range(),
					other.DefinitionRange(),
				),
			)
		} else {
			bound.variables[c.Key.Value] = &Variable{
				definition: c,
			}
		}
	}
	for _, v := range decl.Variables.Entries {
		other, alreadyReferenced := bound.variables[v.Key.Value]
		if alreadyReferenced && other.definition != nil {
			bound.diags = bound.diags.Append(
				duplicateSource(v.Key.Value,
					v.Key.Syntax().Syntax().Range(),
					other.DefinitionRange(),
				),
			)
		} else {
			bound.variables[v.Key.Value] = &Variable{
				definition: v,
			}
			err := bound.bind(v.Value)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, r := range decl.Resources.Entries {
		other, alreadyReferenced := bound.variables[r.Key.Value]
		if alreadyReferenced && other.definition != nil {
			bound.diags = bound.diags.Append(
				duplicateSource(r.Key.Value,
					r.Key.Syntax().Syntax().Range(),
					other.DefinitionRange(),
				),
			)
		} else {
			if err := bound.bindResource(&r); err != nil {
				return nil, err
			}
		}
	}
	for _, o := range decl.Outputs.Entries {
		// Because outputs cannot be referenced, we don't do a referenced check
		other, alreadyDefined := bound.outputs[o.Key.Value]
		if alreadyDefined {
			bound.diags = bound.diags.Append(duplicateSource(o.Key.Value,
				o.Key.Syntax().Syntax().Range(),
				other.Key.Syntax().Syntax().Range()))
		} else {
			bound.outputs[o.Key.Value] = o
			err := bound.bind(o.Value)
			if err != nil {
				return nil, err
			}
		}
	}

	bound.analyzeBindings()
	return bound, nil
}

// Performs analysis on bindings without a schema. This results in missing
// variable errors and unused variable warnings.
func (b *BoundDecl) analyzeBindings() error {
	for name, v := range b.variables {
		if v.definition == nil {
			for _, use := range v.uses {
				b.diags = append(b.diags, variableDoesNotExist(name, use))
			}
		}
		if len(v.uses) == 0 {
			b.diags = append(b.diags, unusedVariable(name, v.DefinitionRange()))
		}
	}
	return nil
}

func (b *BoundDecl) bind(e ast.Expr) error {
	switch e := e.(type) {
	// Primitive types: nothing to bind
	case *ast.NullExpr, *ast.BooleanExpr, *ast.NumberExpr, *ast.StringExpr:

	// Reference types
	case *ast.InterpolateExpr:
		for _, part := range e.Parts {
			if v := part.Value; v != nil {
				err := b.bindPropertyAccess(v, e.Syntax().Syntax().Range())
				if err != nil {
					return err
				}
			}
		}
	case *ast.SymbolExpr:
		return b.bindPropertyAccess(e.Property, e.Syntax().Syntax().Range())

	// Container types:
	case *ast.ListExpr:
		for _, el := range e.Elements {
			err := b.bind(el)
			if err != nil {
				return err
			}
		}
	case *ast.ObjectExpr:
		for _, el := range e.Entries {
			keys := map[string]bool{}
			keyStr, ok := el.Key.(*ast.StringExpr)
			if ok && keys[keyStr.Value] {
				b.diags = append(b.diags, duplicateKey(keyStr.Value, keyStr.Syntax().Syntax().Range()))
			}
			err := b.bind(el.Key)
			if err != nil {
				return err
			}
			err = b.bind(el.Value)
			if err != nil {
				return err
			}
		}

	// Invoke is special, because it requires loading a schema
	case *ast.InvokeExpr:
		return b.bindInvoke(e)

	// Assets and Archives:
	//
	// These take only string expressions.
	// TODO: on a second pass, we could give warnings when they point towards
	// files that don't exist, provide validation for urls provided, ect.
	case *ast.AssetArchiveExpr:
	case *ast.FileArchiveExpr:
	case *ast.FileAssetExpr:
	case *ast.RemoteArchiveExpr:
	case *ast.RemoteAssetExpr:
	case *ast.StringAssetExpr:

	// Stack reference
	case *ast.StackReferenceExpr:

	// Functions
	case *ast.JoinExpr:
		err := b.bind(e.Delimiter)
		if err != nil {
			return err
		}
		return b.bind(e.Values)
	case *ast.SelectExpr:
		err := b.bind(e.Index)
		if err != nil {
			return err
		}
		return b.bind(e.Values)
	case *ast.SplitExpr:
		err := b.bind(e.Delimiter)
		if err != nil {
			return err
		}
		return b.bind(e.Source)
	case *ast.ToBase64Expr:
		return b.bind(e.Value)
	case *ast.ToJSONExpr:
		return b.bind(e.Value)

	case nil:
		// The result of some non-fatal parse errors

	default:
		panic(fmt.Sprintf("Unexpected expr type: '%T'", e))
	}
	return nil
}

func (b *BoundDecl) bindInvoke(invoke *ast.InvokeExpr) error {
	b.invokes[&Invoke{
		token:   invoke.Token.Value,
		defined: invoke,
	}] = struct{}{}
	return nil
}

func (b *BoundDecl) bindResource(r *ast.ResourcesMapEntry) error {
	res := Resource{
		token:   r.Value.Type.Value,
		defined: r,
	}
	entries := map[string]bool{}
	for _, entry := range r.Value.Properties.Entries {
		k := entry.Key.Value
		if entries[k] {
			b.diags = append(b.diags, duplicateKey(k, entry.Key.Syntax().Syntax().Range()))
		}
		if err := b.bind(entry.Key); err != nil {
			return err
		}
		if err := b.bind(entry.Value); err != nil {
			return err
		}
	}
	if err := b.bindResourceOptions(r.Value.Options); err != nil {
		return err
	}
	if other, ok := b.variables[r.Key.Value]; ok {
		b.diags = append(b.diags,
			duplicateSource(
				r.Key.Value,
				r.Key.Syntax().Syntax().Range(),
				other.DefinitionRange()))
	} else {
		b.variables[r.Key.Value] = &Variable{definition: res}
	}
	return nil
}

func (b *BoundDecl) bindResourceOptions(opts ast.ResourceOptionsDecl) error {
	// We only need to bind types that are backed by expressions that could
	// contain variables.
	for _, e := range []ast.Expr{opts.DependsOn, opts.Parent, opts.Provider, opts.Providers} {
		if err := b.bind(e); err != nil {
			return err
		}
	}
	return nil
}

func (b *BoundDecl) bindPropertyAccess(p *ast.PropertyAccess, loc *hcl.Range) error {
	l := p.Accessors
	if v, ok := p.Accessors[0].(*ast.PropertyName); ok {
		b.NewRefernce(v.Name, loc, l[1:])
	} else {
		b.diags = append(b.diags, propertyStartsWithIndex(p, loc))
	}
	return nil
}

func propertyStartsWithIndex(p *ast.PropertyAccess, loc *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  "Property access starts with index",
		Detail:   fmt.Sprintf("Property accesses should start with a bound name: %s", p.String()),
		Subject:  loc,
	}
}

func duplicateSource(name string, subject *hcl.Range, prev *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Duplicate Binding",
		Detail:   fmt.Sprintf("'%s' has already been bound", name),
		Subject:  subject,
		Context:  prev,
	}
}

func duplicateKey(key string, subject *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  "Duplicate key",
		Detail:   fmt.Sprintf("'%s' has already been used as a key in this map", key),
		Subject:  subject,
	}
}

func variableDoesNotExist(name string, use Reference) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf("Missing variable '%s'", name),
		Detail:   fmt.Sprintf("Reference to non-existant variable '%[1]s'. Consider adding a '%[1]s' to the variables section.", name),
		Subject:  use.location,
	}
}

func unusedVariable(name string, loc *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  fmt.Sprintf("Variable '%s' is unused", name),
		Subject:  loc,
	}
}
