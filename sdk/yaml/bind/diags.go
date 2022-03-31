package bind

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
)

func propertyStartsWithIndexDiag(p *ast.PropertyAccess, loc *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  "Property access starts with index",
		Detail:   fmt.Sprintf("Property accesses should start with a bound name: %s", p.String()),
		Subject:  loc,
	}
}

func duplicateSourceDiag(name string, subject *hcl.Range, prev *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Duplicate Binding",
		Detail:   fmt.Sprintf("'%s' has already been bound", name),
		Subject:  subject,
		Context:  prev,
	}
}

func duplicateKeyDiag(key string, subject *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  "Duplicate key",
		Detail:   fmt.Sprintf("'%s' has already been used as a key in this map", key),
		Subject:  subject,
	}
}

func variableDoesNotExistDiag(name string, use Reference) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf("Missing variable '%s'", name),
		Detail:   fmt.Sprintf("Reference to non-existant variable '%[1]s'. Consider adding a '%[1]s' to the variables section.", name),
		Subject:  use.location,
	}
}

func unusedVariableDiag(name string, loc *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  fmt.Sprintf("Variable '%s' is unused", name),
		Subject:  loc,
	}
}

func unparsableTokenDiag(tk string, loc *hcl.Range, err error) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf("Could not parse '%s' as a schema type: %s", tk, err.Error()),
		Detail: "Valid schema tokens are of the form `${pkg}:${module}:${Type}`" +
			" or `${pkg}:${Type}`. Providers take the form `pulumi:providers:${pkg}`",
		Subject: loc,
	}
}

func multipleResourcesDiag(tk string, loc *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  fmt.Sprintf("More then one resource/alias points toward '%s'", tk),
		Detail: "This indicates a problem with the backing schema, not your code." +
			" Contact the package author with this message.",
		Subject: loc,
	}
}

func failedToLoadPackageDiag(pkg string, loc *hcl.Range, err error) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity:    hcl.DiagWarning,
		Summary:     fmt.Sprintf("Failed to load package '%s'", pkg),
		Detail:      fmt.Sprintf("Error: %s", err.Error()),
		Subject:     &hcl.Range{},
		Context:     &hcl.Range{},
		Expression:  nil,
		EvalContext: &hcl.EvalContext{},
	}
}

func missingTokenDiag(pkg, tk string, loc *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf("'%s' doesn't exist in '%s'", tk, pkg),
		Detail:   "",
		Subject:  loc,
	}
}

func depreciatedDiag(item, msg string, loc *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  fmt.Sprintf("'%s' is depreciated", item),
		Detail:   msg,
		Subject:  loc,
	}
}
