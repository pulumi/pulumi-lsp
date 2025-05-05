// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"

	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
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

// ResolveResource resolves an arbitrary resource token into an appropriate schema.Resource.
func resolveResource(c lsp.Client, loader schema.ReferenceLoader, token, version string) (*schema.Resource, error) {
	tokens := strings.Split(token, ":")
	var pkg string
	if len(tokens) < 2 {
		return nil, fmt.Errorf("invalid token '%s': too few spans", token)
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
	schema, err := loader.LoadPackageReference(pkg, v)
	if err != nil {
		return nil, fmt.Errorf("could not resolve resource: %w", err)
	}
	if isProvider {
		return schema.Provider()
	}
	resolvedToken, err := yaml.NewResourcePackage(schema).ResolveResource(token)
	if err != nil {
		return nil, fmt.Errorf("could not resolve resource: %w", err)
	}
	resolvedResource, ok, err := schema.Resources().Get(string(resolvedToken))
	if err != nil {
		return nil, fmt.Errorf("could not resolve resource: internal error: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("could not resolve resource: internal error: "+
			"'%s' resolved to '%s' but the resolved token did not exist",
			token, resolvedToken)
	}
	return resolvedResource, nil
}

// ResolveFunction resolves an arbitrary function token into an appropriate schema.Resource.
func resolveFunction(c lsp.Client, loader schema.ReferenceLoader, token, version string) (*schema.Function, error) {
	tokens := strings.Split(token, ":")
	if len(tokens) < 2 {
		return nil, fmt.Errorf("invalid token '%s': too few spans", token)
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
	schema, err := loader.LoadPackageReference(pkg, v)
	if err != nil {
		return nil, fmt.Errorf("could not resolve function: %w", err)
	}
	resolvedToken, err := yaml.NewResourcePackage(schema).ResolveFunction(token)
	if err != nil {
		return nil, fmt.Errorf("could not resolve function: %w", err)
	}
	resolvedFunction, ok, err := schema.Functions().Get(string(resolvedToken))
	if err != nil {
		return nil, fmt.Errorf("could not resolve function: internal error: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("could not resolve function: internal error: "+
			"'%s' resolved to '%s' but the resolved token did not exist",
			token, resolvedToken)
	}
	return resolvedFunction, nil
}
