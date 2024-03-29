// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"fmt"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"

	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/util"
)

func convertRange(r *hcl.Range) protocol.Range {
	contract.Assertf(r != nil, "Cannot convert an empty range")
	return protocol.Range{
		Start: convertPosition(r.Start),
		End:   convertPosition(r.End),
	}
}

func convertPosition(p hcl.Pos) protocol.Position {
	var defPos hcl.Pos
	var defProto protocol.Position
	if p == defPos {
		return defProto
	}
	contract.Assertf(p.Line != 0, "hcl.Pos line starts at 1")
	return protocol.Position{
		Line:      uint32(p.Line - 1),
		Character: uint32(p.Column - 1),
	}
}

func convertSeverity(s hcl.DiagnosticSeverity) protocol.DiagnosticSeverity {
	switch s {
	case hcl.DiagError:
		return protocol.DiagnosticSeverityError
	case hcl.DiagWarning:
		return protocol.DiagnosticSeverityWarning
	default:
		return protocol.DiagnosticSeverityInformation
	}

}

// Check wheither a lsp position is contained in a yaml range.
func posInRange(r *hcl.Range, pos protocol.Position) bool {
	if r == nil {
		return false
	}
	rng := convertRange(r)
	s := rng.Start
	e := rng.End
	return (posLessThen(s, pos) && posGreaterThen(pos, e)) || pos == s || pos == e
}

// Returns true if p1 < p2
func posGreaterThen(p1, p2 protocol.Position) bool {
	return (p1.Line < p2.Line) ||
		(p1.Line == p2.Line && p1.Character < p2.Character)
}

// Returns true if p1 > p2
func posLessThen(p1, p2 protocol.Position) bool {
	return p1 != p2 && !posGreaterThen(p2, p1)
}

func combineRange(lower, upper protocol.Range) protocol.Range {
	return protocol.Range{
		Start: lower.Start,
		End:   upper.End,
	}
}

// A server level cache for schema loading.
type SchemaCache struct {
	inner schema.ReferenceLoader
	m     *sync.Mutex
	cache map[util.Tuple[string, string]]schema.PackageReference
}

func (c *SchemaCache) Loader(client lsp.Client) schema.ReferenceLoader {
	return SchemaLoader{c, client}
}

type SchemaLoader struct {
	cache *SchemaCache
	c     lsp.Client
}

func (l SchemaLoader) LoadPackageReference(pkg string, version *semver.Version) (schema.PackageReference, error) {
	v := ""
	if version != nil {
		v = version.String()
	}
	load, ok := l.cache.cache[util.Tuple[string, string]{A: pkg, B: v}]
	var err error
	if !ok {
		func() {
			l.cache.m.Lock()
			defer l.cache.m.Unlock()
			load, err = l.cache.inner.LoadPackageReference(pkg, version)
		}()
		if err == nil {
			l.c.LogInfof("Successfully loaded pkg (%s,%s)", pkg, v)
			l.cache.cache[util.Tuple[string, string]{A: pkg, B: v}] = load
		} else {
			l.c.LogErrorf("Failed to load pkg (%s,%s): %s", pkg, v, err.Error())
		}
	}
	return load, err
}

func (l SchemaLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

// ResolveResource resolves an arbitrary resource token into an appropriate schema.Resource.
func (l SchemaCache) ResolveResource(c lsp.Client, token, version string) (*schema.Resource, error) {
	tokens := strings.Split(token, ":")
	var pkg string
	if len(tokens) < 2 {
		return nil, fmt.Errorf("Invalid token '%s': too few spans", token)
	}
	pkg = tokens[0]
	var isProvider bool
	if pkg == "pulumi" {
		isProvider = true
		if tokens[1] == "providers" && len(tokens) > 2 {
			pkg = tokens[2]
		}
	}

	var v *semver.Version
	if version != "" {
		version, err := semver.ParseTolerant(version)
		if err != nil {
			return nil, err
		}
		v = &version
	}
	schema, err := l.Loader(c).LoadPackageReference(pkg, v)
	if err != nil {
		return nil, fmt.Errorf("Could not resolve resource: %w", err)
	}
	if isProvider {
		return schema.Provider()
	}
	resolvedToken, err := yaml.NewResourcePackage(schema).ResolveResource(token)
	if err != nil {
		return nil, fmt.Errorf("Could not resolve resource: %w", err)
	}
	resolvedResource, ok, err := schema.Resources().Get(string(resolvedToken))
	if err != nil {
		return nil, fmt.Errorf("Could not resolve resource: internal error: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("Could not resolve resource: internal error: "+
			"'%s' resolved to '%s' but the resolved token did not exist",
			token, resolvedToken)
	}
	return resolvedResource, nil
}

// ResolveFunction resolves an arbitrary function token into an appropriate schema.Resource.
func (l SchemaCache) ResolveFunction(c lsp.Client, token, version string) (*schema.Function, error) {
	tokens := strings.Split(token, ":")
	if len(tokens) < 2 {
		return nil, fmt.Errorf("Invalid token '%s': too few spans", token)
	}
	pkg := tokens[0]
	var v *semver.Version
	if version != "" {
		version, err := semver.ParseTolerant(version)
		if err != nil {
			return nil, err
		}
		v = &version
	}
	schema, err := l.Loader(c).LoadPackageReference(pkg, v)
	if err != nil {
		return nil, fmt.Errorf("Could not resolve function: %w", err)
	}
	resolvedToken, err := yaml.NewResourcePackage(schema).ResolveFunction(token)
	if err != nil {
		return nil, fmt.Errorf("Could not resolve function: %w", err)
	}
	resolvedFunction, ok, err := schema.Functions().Get(string(resolvedToken))
	if err != nil {
		return nil, fmt.Errorf("Could not resolve function: internal error: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("Could not resolve function: internal error: "+
			"'%s' resolved to '%s' but the resolved token did not exist",
			token, resolvedToken)
	}
	return resolvedFunction, nil
}
