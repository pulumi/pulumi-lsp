package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndentation(t *testing.T) {
	s, blank := indentationLevel("    foo")
	assert.Equal(t, 4, s)
	assert.False(t, blank)
}
