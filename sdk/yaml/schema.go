package yaml

import (
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Find the object at point, as well as it's location. An error indicates that
// there was a problem getting the object at point. If no object is found, all
// zero values are returned.
func (s *document) objectAtPoint(pos protocol.Position) (schema.Type, protocol.Range, error) {
	for _, r := range s.analysis.parsed.Resources.Entries {
		keyRange := r.Key.Syntax().Syntax().Range()
		valueRange := r.Value.Syntax().Syntax().Range()
		if posInRange(keyRange, pos) || posInRange(valueRange, pos) {
			tk := r.Value.Type.Value
			res, err := s.analysis.bound.GetResources(tk)
			if err != nil {
				return nil, protocol.Range{}, err
			}
			if len(res) == 0 {
				return nil, protocol.Range{}, nil
			}
			return &schema.ResourceType{
				Token:    tk,
				Resource: res[0].Schema(),
			}, combineRange(convertRange(keyRange), convertRange(valueRange)), nil
		}
	}
	return nil, protocol.Range{}, nil
}

func describeType(t schema.Type) protocol.MarkupContent {
	markdown := func(body string) protocol.MarkupContent {
		return protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: body,
		}
	}
	plain := func(body string, args ...interface{}) protocol.MarkupContent {
		return protocol.MarkupContent{
			Kind:  protocol.PlainText,
			Value: fmt.Sprintf(body, args...),
		}
	}
	if schema.IsPrimitiveType(t) {
		return plain("%s (primitive)", t)
	}
	switch t := t.(type) {
	case *schema.ResourceType:
		body := "## Resource: " + t.Token + "\n"
		if t.Resource != nil {
			body += "\n### Inputs:\n"
			for _, input := range t.Resource.InputProperties {
				body += "\t" + input.Name + ": " + input.Type.String() + "\n"
			}
			body += "\n### Outputs:\n"
			for _, input := range t.Resource.Properties {
				body += "\t" + input.Name + ": " + input.Type.String() + "\n"
			}
		}
		return markdown(body)
	default:
		return plain("unknown type %s", t)
	}
}
