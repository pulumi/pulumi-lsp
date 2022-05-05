// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"fmt"

	"go.lsp.dev/protocol"
)

// Find the object at point, as well as it's location. An error indicates that
// there was a problem getting the object at point. If no object is found, all
// zero values are returned.
func (doc *document) objectAtPoint(pos protocol.Position) (Object, error) {
	parsed, ok := doc.analysis.parsed.GetResult()
	if !ok {
		return nil, fmt.Errorf("Could not get parsed schema: canceled")
	}
	if parsed.A == nil {
		return nil, fmt.Errorf("Could not get parsed schema: nil result")
	}
	for _, r := range parsed.A.Resources.Entries {
		keyRange := r.Key.Syntax().Syntax().Range()
		if r.Value != nil && r.Value.Type != nil && posInRange(r.Value.Type.Syntax().Syntax().Range(), pos) {
			tk := r.Value.Type.Value
			bound, ok := doc.analysis.bound.GetResult()
			if !ok {
				return nil, fmt.Errorf("Could not get bound schema: canceled")
			}
			if bound.A == nil {
				return nil, fmt.Errorf("COuld not get bound schema: nil result")
			}
			res, err := bound.A.GetResources(tk)
			if err != nil {
				return nil, err
			}
			if len(res) == 0 {
				return nil, nil
			}
			valueRange := r.Value.Syntax().Syntax().Range()
			return Resource{
				object: object{combineRange(convertRange(keyRange), convertRange(valueRange))},
				schema: res[0].Schema(),
			}, nil
		}
	}
	bound, ok := doc.analysis.bound.GetResult()
	if !ok {
		return nil, fmt.Errorf("Could not get bound schema: canceled")
	}
	if bound.A == nil {
		return nil, fmt.Errorf("COuld not get bound schema: nil result")
	}

	for _, f := range bound.A.Invokes() {
		tk := f.Expr().Token
		if posInRange(tk.Syntax().Syntax().Range(), pos) {
			return Invoke{
				object: object{convertRange(f.Expr().Syntax().Syntax().Range())},
				schema: f.Schema(),
			}, nil
		}
	}

	for _, r := range bound.A.References() {
		if posInRange(r.Range(), pos) {
			return &Reference{
				object: object{convertRange(r.Range())},
				ref:    &r,
			}, nil
		}
	}
	return nil, nil
}
