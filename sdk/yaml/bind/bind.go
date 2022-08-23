// Copyright 2022, Pulumi Corporation.  All rights reserved.

// Bind performs static analysis on a YAML document. The entry point for the package is `NewDecl`.
//
// bind.go contains logic for the initial non-schema binding of an `ast.TemplateDecl` into a `Decl`.
// query.go contains logic to get information out of a `Decl`.
// schema.go handles loading appropriate schemas and binding them to an existing `Decl`.
// diags.go contains the diagnostic error messages used.
package bind

import (
	"fmt"
	"sync"

	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi-lsp/sdk/util"
)

// A bound template.
//
// NOTE: the binding need not be complete, and the template need not be valid.
type Decl struct {
	// All names in the global namespace.
	variables map[string]*Variable

	// The output namespace
	outputs map[string]ast.PropertyMapEntry

	// The set of all invokes.
	invokes map[*Invoke]struct{}

	diags hcl.Diagnostics

	loadedPackages map[string]pkgCache

	lock *sync.RWMutex
}

// A value that a reference can bind to. This includes the variables, config and
// resources section of the yaml template.
type Variable struct {
	// definition is either a Resource, *ast.ConfigParamDecl
	definition Definition
	uses       []Reference

	// The variable name in the global namespace
	name string
}

func (v *Variable) Name() string {
	return v.name
}

func (v *Variable) Source() Definition {
	return v.definition
}

type Definition interface {
	ResolvableType
	DefinitionRange() *hcl.Range
	isDefinition()
}

type Reference struct {
	location *hcl.Range
	access   PropertyAccessorList

	variable *Variable
	// A cache of how the variable is represented in text
	s string
}

// Returns the property accessors that hang on the variable referenced.
// Examples:
//   ${foo.bar}     => ["bar"]
//   ${foo.bar.baz} => ["bar" "baz"]
//   ${foo.}        => []
//   ${foo}         => []
func (r *Reference) Accessors() PropertyAccessorList {
	return r.access
}

func (r *Reference) Var() *Variable {
	return r.variable
}

func (r *Reference) Range() *hcl.Range {
	return r.location
}

func (r *Reference) String() string {
	return r.s
}

type PropertyAccessorList []PropertyAccessor

// Compute out the type chain as long as possible. The completion chain always
// has 1 element since it includes the root.
func (l PropertyAccessorList) TypeFromRoot(root schema.Type) ([]schema.Type, *hcl.Diagnostic) {
	types := []schema.Type{root}
	handleProperties := func(tag string, props []*schema.Property, parent schema.Type, rnge *hcl.Range) (*schema.Property, *hcl.Diagnostic) {
		existing := map[string]struct{}{}
		for _, p := range props {
			if p.Name == tag {
				return p, nil
			}
			existing[p.Name] = struct{}{}
		}
		return nil, propertyDoesNotExistDiag(tag, parent.String(), util.MapKeys(existing), rnge)
	}

	getStringTag := func(p ast.PropertyAccessor) string {
		switch a := p.(type) {
		case *ast.PropertyName:
			return a.Name
		case *ast.PropertySubscript:
			switch a := a.Index.(type) {
			case string:
				return a
			}
		}
		return ""
	}
	exit := func(diag *hcl.Diagnostic) ([]schema.Type, *hcl.Diagnostic) {
		for len(types)+1 < len(l) {
			// Indicate the a type exists but could not be found
			types = append(types, nil)
		}
		return types, diag
	}
	for _, prop := range l {
		next := func(t schema.Type) {
			types = append(types, t)
			root = t
		}
		switch typ := codegen.UnwrapType(root).(type) {
		case *schema.ArrayType:
			if s, ok := prop.PropertyAccessor.(*ast.PropertySubscript); ok {
				if _, ok := s.Index.(int); ok {
					next(typ.ElementType)
					continue
				}
				// TODO: specialize for map index
				return exit(noPropertyAccessDiag(typ.String(), prop.rnge))
			}
			return exit(noPropertyAccessDiag(typ.String(), prop.rnge))

		case *schema.MapType:
			switch a := prop.PropertyAccessor.(type) {
			case *ast.PropertySubscript:
				_, name := a.Index.(string)
				if name {
					next(typ.ElementType)
					continue
				}
				return exit(noPropertyIndexDiag(typ.String(), prop.rnge))
			case *ast.PropertyName:
				next(typ.ElementType)
				continue
			}

		case *schema.ResourceType:
			r := typ.Resource
			tag := getStringTag(prop.PropertyAccessor)
			if tag == "" {
				return exit(noPropertyIndexDiag(typ.String(), prop.rnge))
			}
			if r == nil {
				return exit(nil)
			}

			prop, diag := handleProperties(tag, util.ResourceProperties(r), typ, prop.rnge)
			if diag != nil {
				return exit(diag)
			}
			next(prop.Type)

		case *schema.ObjectType:
			tag := getStringTag(prop.PropertyAccessor)
			if tag == "" {
				return exit(noPropertyIndexDiag(typ.String(), prop.rnge))
			}
			prop, diag := handleProperties(tag, typ.Properties, typ, prop.rnge)
			if diag != nil {
				return exit(diag)
			}
			next(prop.Type)
		}
	}
	return exit(nil)
}

type PropertyAccessor struct {
	ast.PropertyAccessor

	rnge *hcl.Range
}

func (b *Decl) newRefernce(variable string, loc *hcl.Range, accessor []ast.PropertyAccessor, repr string) {
	v, ok := b.variables[variable]
	// Name is used for the initial offset
	var l []PropertyAccessor
	offsetByte := len(variable) + loc.Start.Byte
	offsetChar := len(variable) + loc.Start.Column
	incrOffset := func(i int) {
		offsetByte += i
		offsetChar += i
	}
	// 2 for the leading ${
	incrOffset(2)
	for _, a := range accessor {
		length := 0
		postOffset := 0
		switch a := a.(type) {
		case *ast.PropertyName:
			incrOffset(1) // 1 for the .
			length = len(a.Name)
		case *ast.PropertySubscript:
			switch a := a.Index.(type) {
			case string:
				incrOffset(2)
				postOffset = 2
				length = len(a)
			case int:
				incrOffset(1)
				postOffset = 1
				length = len(fmt.Sprintf("%d", a)) + 2 // 2 brackets
			}
		}
		aRange := loc
		if line := loc.Start.Line; line == loc.End.Line {
			start := hcl.Pos{
				Line:   line,
				Column: offsetChar,
				Byte:   offsetByte,
			}
			aRange = &hcl.Range{
				Filename: loc.Filename,
				Start:    start,
				End: hcl.Pos{
					Line:   line,
					Column: offsetChar + length,
					Byte:   offsetByte + length,
				},
			}
			incrOffset(length + postOffset)
		}
		l = append(l, PropertyAccessor{
			PropertyAccessor: a,
			rnge:             aRange,
		})
	}
	ref := Reference{location: loc, access: l}
	if !ok {
		v = &Variable{name: variable, uses: []Reference{}}
		b.variables[variable] = v
	}
	ref.variable = v
	ref.s = repr
	v.uses = append(v.uses, ref)

}

type Resource struct {
	token      string
	defined    *ast.ResourcesMapEntry
	definition *schema.Resource
}

func (r *Resource) isDefinition() {}
func (r *Resource) DefinitionRange() *hcl.Range {
	return r.defined.Key.Syntax().Syntax().Range()
}
func (r *Resource) ResolveType(*Decl) schema.Type {
	return &schema.ResourceType{
		Token:    r.token,
		Resource: r.definition,
	}
}

func (r *Resource) Schema() *schema.Resource {
	return r.definition
}

type VariableMapEntry struct{ ast.VariablesMapEntry }

func (r *VariableMapEntry) isDefinition() {}
func (v *VariableMapEntry) DefinitionRange() *hcl.Range {
	s := v.Key.Syntax()
	if s == nil {
		return nil
	}
	ds := s.Syntax()
	if ds == nil {
		return nil
	}
	return ds.Range()
}

type ConfigMapEntry struct{ ast.ConfigMapEntry }

func (c *ConfigMapEntry) isDefinition() {}
func (c *ConfigMapEntry) DefinitionRange() *hcl.Range {
	return c.Key.Syntax().Syntax().Range()
}

type ResolvableType interface {
	ResolveType(*Decl) schema.Type
}

func (c *ConfigMapEntry) ResolveType(*Decl) schema.Type {
	switch c.Value.Type.Value {
	case "string":
		return schema.StringType
	case "int":
		return schema.IntType
	default:
		return nil
	}
}

func (v *VariableMapEntry) ResolveType(d *Decl) schema.Type {
	return d.typeExpr(v.Value)
}

type Invoke struct {
	token      string
	defined    *ast.InvokeExpr
	definition *schema.Function
}

func (f *Invoke) Expr() *ast.InvokeExpr {
	return f.defined
}

func (f *Invoke) Schema() *schema.Function {
	return f.definition
}

func NewDecl(decl *ast.TemplateDecl) (*Decl, error) {
	bound := &Decl{
		variables: map[string]*Variable{
			pulumiyaml.PulumiVarName: {
				definition: &VariableMapEntry{
					ast.VariablesMapEntry{
						Key: &ast.StringExpr{},
						Value: ast.Object(
							ast.ObjectProperty{Key: ast.String("cwd"), Value: ast.String("cwd")},
							ast.ObjectProperty{Key: ast.String("stack"), Value: ast.String("stack")},
							ast.ObjectProperty{Key: ast.String("project"), Value: ast.String("project")},
						),
					},
				},
				uses: nil,
				name: pulumiyaml.PulumiVarName,
			},
		},
		outputs:        map[string]ast.PropertyMapEntry{},
		invokes:        map[*Invoke]struct{}{},
		diags:          hcl.Diagnostics{},
		loadedPackages: map[string]pkgCache{},
		lock:           &sync.RWMutex{},
	}

	for _, c := range decl.Configuration.Entries {
		other, alreadyReferenced := bound.variables[c.Key.Value]
		if alreadyReferenced && other.definition != nil {
			bound.diags = bound.diags.Append(
				duplicateSourceDiag(c.Key.Value,
					c.Key.Syntax().Syntax().Range(),
					other.definition.DefinitionRange(),
				),
			)
		} else {
			bound.variables[c.Key.Value] = &Variable{
				name:       c.Key.Value,
				definition: &ConfigMapEntry{c},
			}
		}
	}
	for _, v := range decl.Variables.Entries {
		other, alreadyReferenced := bound.variables[v.Key.Value]
		if alreadyReferenced && other.definition != nil {
			bound.diags = bound.diags.Append(
				duplicateSourceDiag(v.Key.Value,
					v.Key.Syntax().Syntax().Range(),
					other.definition.DefinitionRange(),
				),
			)
		} else {
			bound.variables[v.Key.Value] = &Variable{
				name:       v.Key.Value,
				definition: &VariableMapEntry{v},
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
			var subject *hcl.Range
			if s := r.Key.Syntax(); s != nil && s.Syntax() != nil {
				subject = s.Syntax().Range()
			}
			var previous *hcl.Range
			if other != nil && other.definition != nil {
				previous = other.definition.DefinitionRange()
			}
			bound.diags = bound.diags.Append(
				duplicateSourceDiag(r.Key.Value,
					subject, previous,
				),
			)
		} else {
			if err := bound.bindResource(r); err != nil {
				return nil, err
			}
		}
	}
	for _, o := range decl.Outputs.Entries {
		// Because outputs cannot be referenced, we don't do a referenced check
		other, alreadyDefined := bound.outputs[o.Key.Value]
		if alreadyDefined {
			bound.diags = bound.diags.Append(duplicateSourceDiag(o.Key.Value,
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

	err := bound.analyzeBindings()
	return bound, err
}

// Performs analysis on bindings without a schema. This results in missing
// variable errors and unused variable warnings.
func (b *Decl) analyzeBindings() error {
	for name, v := range b.variables {
		if v.definition == nil {
			for _, use := range v.uses {
				b.diags = append(b.diags, variableDoesNotExistDiag(name, use))
			}
		}
		switch v.definition.(type) {
		case *VariableMapEntry:
			if len(v.uses) == 0 && name != pulumiyaml.PulumiVarName {
				b.diags = append(b.diags, unusedVariableDiag(name, v.definition.DefinitionRange()))
			}
		}
	}
	return nil
}

func (b *Decl) bind(e ast.Expr) error {
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
				b.diags = append(b.diags, duplicateKeyDiag(keyStr.Value, keyStr.Syntax().Syntax().Range()))
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

	case *ast.ReadFileExpr:
		return b.bind(e.Path)

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

func (d *Decl) bindInvoke(invoke *ast.InvokeExpr) error {
	d.invokes[&Invoke{
		token:   invoke.Token.Value,
		defined: invoke,
	}] = struct{}{}
	return d.bind(invoke.Args())
}

func (d *Decl) bindResource(r ast.ResourcesMapEntry) error {
	if r.Value == nil {
		d.variables[r.Key.Value] = &Variable{definition: &Resource{defined: &r}, name: r.Key.Value}
		d.diags = append(d.diags, missingResourceBodyDiag(r.Key.Value, r.Key.Syntax().Syntax().Range()))
		return nil
	}
	if r.Value.Type == nil {
		d.variables[r.Key.Value] = &Variable{definition: &Resource{defined: &r}, name: r.Key.Value}
		d.diags = append(d.diags, missingResourceTypeDiag(r.Key.Value, r.Key.Syntax().Syntax().Range()))
		return nil
	}
	res := Resource{
		token:   r.Value.Type.Value,
		defined: &r,
	}
	entries := map[string]bool{}
	for _, entry := range r.Value.Properties.Entries {
		k := entry.Key.Value
		if entries[k] {
			d.diags = append(d.diags, duplicateKeyDiag(k, entry.Key.Syntax().Syntax().Range()))
		}
		if err := d.bind(entry.Key); err != nil {
			return err
		}
		if err := d.bind(entry.Value); err != nil {
			return err
		}
	}
	if err := d.bindResourceOptions(r.Value.Options); err != nil {
		return err
	}
	d.variables[r.Key.Value] = &Variable{definition: &res, name: r.Key.Value}
	return nil
}

func (b *Decl) bindResourceOptions(opts ast.ResourceOptionsDecl) error {
	// We only need to bind types that are backed by expressions that could
	// contain variables.
	for _, e := range []ast.Expr{opts.DependsOn, opts.Parent, opts.Provider, opts.Providers} {
		if err := b.bind(e); err != nil {
			return err
		}
	}
	return nil
}

func (b *Decl) bindPropertyAccess(p *ast.PropertyAccess, loc *hcl.Range) error {
	l := p.Accessors
	if len(l) == 0 {
		b.diags = append(b.diags, emptyPropertyAccessDiag(loc))
		// We still take the reference so we can lookup this interpolation later.
		b.newRefernce("", loc, nil, p.String())
		return nil
	}
	if v, ok := p.Accessors[0].(*ast.PropertyName); ok {
		b.newRefernce(v.Name, loc, l[1:], p.String())
	} else {
		b.diags = append(b.diags, propertyStartsWithIndexDiag(p, loc))
		// We still take the reference so we can lookup this interpolation later.
		b.newRefernce("", loc, nil, p.String())
	}
	return nil
}
