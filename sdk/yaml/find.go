// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-lsp/sdk/lsp"
	"github.com/pulumi/pulumi-lsp/sdk/util"
	"go.lsp.dev/protocol"
)

type UnparsableError struct {
	msg string

	Bail bool // If an attempt should be made to continue without the parsed schema
}

func (e UnparsableError) Error() string {
	var post string
	if e.msg != "" {
		post = ": " + e.msg
	}
	return fmt.Sprintf("could not get parsed schema%s", post)
}

// Find the object at point, as well as it's location. An error indicates that
// there was a problem getting the object at point. If no object is found, all
// zero values are returned.
func (doc *document) objectAtPoint(pos protocol.Position) (Object, error) {
	parsed, ok := doc.analysis.parsed.GetResult()
	canceledErr := UnparsableError{"canceled", true}
	nilError := UnparsableError{"failed", false}
	if !ok {
		return nil, canceledErr
	}
	if parsed.A == nil {
		return nil, nilError
	}
	for _, r := range parsed.A.Resources.Entries {
		keyRange := r.Key.Syntax().Syntax().Range()
		if r.Value != nil && r.Value.Type != nil && posInRange(r.Value.Type.Syntax().Syntax().Range(), pos) {
			tk := r.Value.Type.Value
			bound, ok := doc.analysis.bound.GetResult()
			if !ok {
				return nil, canceledErr
			}
			if bound.A == nil {
				return nil, nilError
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
		return nil, canceledErr
	}
	if bound.A == nil {
		return nil, nilError
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

type KeyPos = util.Tuple[protocol.Position, string]

// Return the place where the enclosing object starts
func enclosingKey(text lsp.Document, pos protocol.Position) (protocol.Position, bool, error) {
	lineNum := int(pos.Line)
	line, err := text.Line(lineNum)
	if err != nil {
		return protocol.Position{}, false, err
	}
	indentation, _ := indentationLevel(line)
	// Scan up until a non-blank line with less indentation is found. This
	// assumes that the YAML is valid and not in flow form.
	for lineNum > 1 {
		lineNum--
		line, err := text.Line(lineNum)
		if err != nil {
			return protocol.Position{}, false, err
		}
		ind, blank := indentationLevel(line)
		if !blank && ind < indentation && strings.HasSuffix(strings.TrimSpace(line), ":") {
			// Found the parent
			return protocol.Position{
				Line:      uint32(lineNum),
				Character: uint32(ind),
			}, true, nil
		}
	}
	// We didn't find anything
	return protocol.Position{}, false, nil
}

// Return the chain of parent keys from most senior to least senior.
func parentKeys(text lsp.Document, pos protocol.Position) ([]KeyPos, bool, error) {
	parent, ok, err := enclosingKey(text, pos)
	if err != nil || !ok {
		return nil, ok, err
	}
	key, err := text.Line(int(parent.Line))
	if err != nil {
		return nil, false, err
	}
	key = strings.TrimSpace(key)
	key = strings.TrimSuffix(key, ":")

	grandparents, ok, err := parentKeys(text, parent)
	if err != nil {
		return nil, false, err
	}
	tup := util.Tuple[protocol.Position, string]{A: parent, B: key}
	if !ok {
		return []KeyPos{tup}, true, nil
	}
	return append(grandparents, tup), true, nil
}

// Find the number of leading spaces in a line.
func indentationLevel(line string) (spaces int, allBlank bool) {
	level := 0
	for _, c := range line {
		if c != ' ' {
			break
		}
		level += 1
	}
	return level, strings.TrimSpace(line) == ""
}

// subsidiaryKeys returns a map of subsidiary keys to their positions.
func subsidiaryKeys(text lsp.Document, pos protocol.Position) (map[string]protocol.Position, error) {
	line, err := text.Line(int(pos.Line))
	if err != nil {
		return nil, err
	}
	topLevel, blank := indentationLevel(line)
	if blank {
		return nil, fmt.Errorf("Cannot call subsidiaryKeys on a blank line")
	}
	level := -1
	m := map[string]protocol.Position{}
	for i := int(pos.Line) + 1; i < text.LineLen(); i++ {
		line, err := text.Line(i)
		if err != nil {
			return nil, err
		}
		indLevel, blank := indentationLevel(line)
		if blank {
			continue
		}
		if indLevel <= topLevel {
			// Found a key at the same or greater level. We are done.
			break
		}
		if level == -1 {
			// Our first key indicates the indentation level of subsidiary keys
			level = indLevel
		}
		if indLevel == level {
			keyValue := strings.Split(line, ":")
			if len(keyValue) == 0 {
				continue
			}
			m[strings.TrimSpace(keyValue[0])] = protocol.Position{
				Line:      uint32(i),
				Character: uint32(indLevel),
			}
		} else if indLevel < level {
			// Invalid yaml:
			// foo:
			//   bar: valid, level = 2
			//  buz: invalid, level = 1
		}
	}
	return m, nil
}

// siblingKeys returns list of properties at the level of `pos`.
func siblingKeys(text lsp.Document, pos protocol.Position) (map[string]protocol.Position, bool, error) {
	parent, ok, err := enclosingKey(text, pos)
	if err != nil || !ok {
		return nil, ok, err
	}
	siblings, err := subsidiaryKeys(text, parent)
	if err != nil {
		return nil, false, err
	}
	return siblings, true, nil
}
