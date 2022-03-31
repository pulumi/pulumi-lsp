package yaml

import (
	"bytes"
	"fmt"
	"io"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
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
		tk := r.Value.Type
		keyRange := r.Key.Syntax().Syntax().Range()
		valueRange := r.Value.Syntax().Syntax().Range()
		if posInRange(tk.Syntax().Syntax().Range(), pos) {
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
	return nil, nil
}

type object struct {
	rnge protocol.Range
}

func (object) isObject() {}
func (o object) Range() *protocol.Range {
	return &o.rnge
}

// An object in the schema that can be acted upon.
type Object interface {
	Describe() protocol.MarkupContent
	Range() *protocol.Range
	isObject()
}

type Resource struct {
	object
	schema *schema.Resource
}

func (r Resource) Describe() protocol.MarkupContent {
	b := &bytes.Buffer{}
	writeResource(b, r.schema)
	return protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: b.String(),
	}
}

type Invoke struct {
	object
	schema *schema.Function
}

func (f Invoke) Describe() protocol.MarkupContent {
	b := &bytes.Buffer{}
	writeFunction(b, f.schema)
	return protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: b.String(),
	}
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
		b := &bytes.Buffer{}
		writeResource(b, t.Resource)
		return markdown(b.String())
	default:
		return plain("unknown type %s", t)
	}
}

type Writer = func(msg string, args ...interface{})

func MakeIOWriter[T any](f func(Writer, T)) func(io.Writer, T) {
	return func(w io.Writer, t T) {
		f(func(format string, a ...interface{}) {
			fmt.Fprintf(w, format, a...)
		}, t)
	}
}

var writeFunction = MakeIOWriter(func(w Writer, f *schema.Function) {
	w("# Function: %s\n", f.Token)
	w("\n%s\n", f.Comment)
	if f.DeprecationMessage != "" {
		w("## Depreciated\n%s\n", f.DeprecationMessage)
	}

	if f.Inputs != nil {
		w("## Arguments\n")
		w("**Type:** `%s`\n", f.Inputs.Token)
		for _, input := range f.Inputs.Properties {
			writePropertyDescription(w, input)
		}
	}
	if f.Outputs != nil {
		w("## Return\n")
		w("**Type:** `%s`\n", f.Outputs.Token)
		for _, out := range f.Outputs.Properties {
			writePropertyDescription(w, out)
		}
	}

})

var writeResource = MakeIOWriter(func(w Writer, r *schema.Resource) {
	w("# Resource: %s\n", r.Token)
	w("\n%s\n", r.Comment)
	if r.DeprecationMessage != "" {
		w("## Depreciated\n%s\n", r.DeprecationMessage)
	}
	w("## Inputs\n")
	for _, input := range r.InputProperties {
		writePropertyDescription(w, input)
	}
	w("## Outputs\n")
	for _, output := range r.Properties {
		writePropertyDescription(w, output)
	}
})

func writePropertyDescription(w Writer, prop *schema.Property) {
	w("### %s\n", prop.Name)
	w("**Type:** `%s`\n\n", prop.Type)
	w("%s\n", prop.Comment)
}

func writeObjectDescription(w Writer, obj *schema.ObjectType) {

}
