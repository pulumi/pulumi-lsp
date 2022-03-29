package yaml

import (
	"github.com/hashicorp/hcl/v2"
	"go.lsp.dev/protocol"
)

func convertRange(r *hcl.Range) protocol.Range {
	return protocol.Range{
		Start: convertPosition(r.Start),
		End:   convertPosition(r.End),
	}
}

func convertPosition(p hcl.Pos) protocol.Position {
	return protocol.Position{
		Line:      uint32(p.Line),
		Character: uint32(p.Column),
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
