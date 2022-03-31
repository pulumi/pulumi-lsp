package yaml

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"go.lsp.dev/protocol"
)

func TestPosInRange(t *testing.T) {
	loc := &hcl.Range{
		Start: hcl.Pos{Column: 17, Line: 3},
		End:   hcl.Pos{Column: 26, Line: 17},
	}

	pos := protocol.Position{Line: 5, Character: 12}
	assert.False(t, posInRange(loc, pos))
}
