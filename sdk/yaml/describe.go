// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"bytes"
	"fmt"
	"io"

	"go.lsp.dev/protocol"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-lsp/sdk/yaml/bind"
)

type object struct {
	rnge protocol.Range
}

func (object) isObject() {}
func (o object) Range() *protocol.Range {
	return &o.rnge
}

// An object in the schema that can be acted upon.
type Object interface {
	Describe() (protocol.MarkupContent, bool)
	Range() *protocol.Range
	isObject()
}

type Reference struct {
	object
	ref *bind.Reference
}

func (r *Reference) Describe() (protocol.MarkupContent, bool) {
	return protocol.MarkupContent{}, false
}

type Resource struct {
	object
	schema *schema.Resource
}

func (r Resource) Describe() (protocol.MarkupContent, bool) {
	if r.schema == nil {
		return protocol.MarkupContent{}, false
	}

	b := &bytes.Buffer{}
	writeResource(b, r.schema)
	return protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: b.String(),
	}, true
}

type Invoke struct {
	object
	schema *schema.Function
}

func (f Invoke) Describe() (protocol.MarkupContent, bool) {
	if f.schema == nil {
		return protocol.MarkupContent{}, false
	}
	b := &bytes.Buffer{}
	writeFunction(b, f.schema)
	return protocol.MarkupContent{
		Kind:  protocol.Markdown,
		Value: b.String(),
	}, true
}

type Writer = func(msg string, args ...interface{})

func MakeIOWriter[T any](f func(Writer, T)) func(io.Writer, T) {
	return func(w io.Writer, t T) {
		f(func(format string, a ...interface{}) {
			_, err := fmt.Fprintf(w, format, a...)
			contract.IgnoreError(err)
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
	w("**Type:** `%s`\n\n", codegen.UnwrapType(prop.Type))
	w("%s\n", prop.Comment)
}
