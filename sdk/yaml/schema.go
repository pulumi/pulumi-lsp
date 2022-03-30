package yaml

import (
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type schemaHandler struct {
	loader schema.Loader
}

// Find the object at point, as well as it's location. An error indicates that
// there was a problem getting the object at point. If no object is found, all
// zero values are returned.
func objectAtPoint(template *ast.TemplateDecl, pos protocol.Position) (schema.Type, protocol.Range, error) {
	for _, r := range template.Resources.Entries {
		keyRange := r.Key.Syntax().Syntax().Range()
		valueRange := r.Value.Syntax().Syntax().Range()
		if posInRange(keyRange, pos) || posInRange(valueRange, pos) {
			return &schema.ResourceType{Token: r.Key.Value}, combineRange(convertRange(keyRange), convertRange(valueRange)), nil
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
