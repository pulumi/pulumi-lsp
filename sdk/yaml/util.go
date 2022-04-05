// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"sync"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"

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
	inner schema.Loader
	c     lsp.Client
	m     *sync.Mutex
	cache map[util.Tuple[string, string]]*schema.Package
}

func (c *SchemaCache) Loader(client lsp.Client) schema.Loader {
	return SchemaLoader{c, client}
}

type SchemaLoader struct {
	cache *SchemaCache
	c     lsp.Client
}

func (l SchemaLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	v := ""
	if version != nil {
		v = version.String()
	}
	l.c.LogDebugf("Loading package (%s,%s) ", pkg, v)
	load, ok := l.cache.cache[util.Tuple[string, string]{A: pkg, B: v}]
	var err error
	if ok {
		l.c.LogDebugf("Returning cached pkg (%s,%s)", pkg, v)
	} else {
		func() {
			l.cache.m.Lock()
			defer l.cache.m.Unlock()
			load, err = l.cache.inner.LoadPackage(pkg, version)
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
