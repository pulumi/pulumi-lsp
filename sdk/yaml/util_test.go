// Copyright 2022, Pulumi Corporation.  All rights reserved.

package yaml

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"go.lsp.dev/protocol"
)

func TestPosInRange(t *testing.T) {
	loc := &hcl.Range{
		Start: hcl.Pos{Line: 3, Column: 17},
		End:   hcl.Pos{Line: 17, Column: 26},
	}

	pos := protocol.Position{Line: 5, Character: 12}
	assert.True(t, posInRange(loc, pos))
}
