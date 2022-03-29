package lsp

import "go.lsp.dev/protocol"

// A text document that can handle incremental updates.
type Text struct {
	text string
}

func NewText(item protocol.TextDocumentItem) Text {
	panic("unimplimented")
}
