package lsp

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"go.lsp.dev/protocol"
)

// A thread-safe text document designed to handle incremental updates.
type Document struct {
	// Any method that reads `lines` needs to acquire a read lock of `m`. To
	// mutate `lines`, a write lock is required.
	//
	// TODO: find/implement a rope
	lines []string
	// NOTE: uri should be considered immutable. This allows us to fetch is
	// without a lock.
	uri protocol.DocumentURI
	m   *sync.RWMutex

	version    int32
	languageID protocol.LanguageIdentifier
}

func NewDocument(item protocol.TextDocumentItem) Document {
	return Document{
		lines:      strings.Split(item.Text, lineDeliminator),
		uri:        item.URI,
		version:    item.Version,
		languageID: item.LanguageID,

		m: new(sync.RWMutex),
	}
}

func (t *Document) AcceptChanges(changes []protocol.TextDocumentContentChangeEvent) error {
	t.m.Lock()
	defer t.m.Unlock()
	for _, change := range changes {
		err := t.acceptChange(change)
		if err != nil {
			return err
		}
	}
	return nil
}

const lineDeliminator = "\n"

func (t *Document) URI() protocol.DocumentURI {
	return t.uri
}

// Returns the whole document as a string.
func (t *Document) String() string {
	t.m.RLock()
	defer t.m.RUnlock()
	return strings.Join(t.lines, lineDeliminator)
}

// Window provides the text of the document that fits in the window.
func (t *Document) Window(window protocol.Range) (string, error) {
	// This is only the range, which was passed by value
	if err := validateRange(window); err != nil {
		return "", err
	}
	t.m.RLock()
	defer t.m.RUnlock()
	if err := t.validateRange(window); err != nil {
		return "", err
	}
	sLine := int(window.Start.Line)
	sChar := int(window.Start.Character)
	eLine := int(window.End.Line)
	eChar := int(window.End.Character)
	if window.Start.Line == window.End.Line {
		return t.lines[sLine][sChar:eChar], nil
	}
	return t.lines[sLine][sChar:] + strings.Join(t.lines[sLine+1:eLine], lineDeliminator) + t.lines[eLine][:eChar], nil
}

// Validate that the range is in the Text. Calling validateRange requires
// holding any lock on the document.
func (t *Document) validateRange(r protocol.Range) error {
	sLine := int(r.Start.Line)
	sChar := int(r.Start.Character)
	if sLine >= len(t.lines) {
		return newInvalidRange(r, "start line %d out of bounds for document with %d lines", sLine, len(t.lines))
	}
	if sChar >= len(t.lines[sLine]) {
		return newInvalidRange(r, "start character %d out of bound on line %d", sChar, sLine)
	}
	eLine := int(r.End.Line)
	eChar := int(r.End.Character)
	if eLine >= len(t.lines) {
		return newInvalidRange(r, "end line %d out of bounds for document with %d lines", eLine, len(t.lines))
	}
	if eChar >= len(t.lines[eLine]) {
		return newInvalidRange(r, "end character %d out of bound on line %d", eChar, eLine)
	}
	return nil
}

// acceptChange implements the change on the document. Calling acceptChange
// correctly requires holding a write lock on the document.
func (t *Document) acceptChange(change protocol.TextDocumentContentChangeEvent) error {
	var defRange protocol.Range
	if change.Range == defRange && change.RangeLength == 0 {
		// This indicates that the whole document should be changed.
		t.lines = strings.Split(change.Text, lineDeliminator)
		return nil
	}
	// Note: RangeLength is depreciated
	lines := strings.Split(change.Text, lineDeliminator)
	s := change.Range.Start
	e := change.Range.End
	contract.Assert(len(lines) != 0)
	if s.Line == e.Line {
		l := t.lines[s.Line]
		if len(lines) == 1 {
			// We can just change the one line
			t.lines[s.Line] = l[:s.Character] + change.Text + l[e.Character:]
			return nil
		}
		// We need to add new lines
		end := l[e.Character:]
		start := l[:s.Character] + lines[0]
		lines[len(lines)-1] += end
		t.lines = append(append(append(t.lines[:s.Line], start), lines...), t.lines[e.Line:]...)
		return nil
	}
	// Range is across multiple lines
	if len(lines) == 1 {
		// Joining lines together
		t.lines[s.Line] = t.lines[s.Line][:s.Character] + lines[0] + t.lines[e.Line][e.Character:]
		t.lines = append(t.lines[:s.Line+1], t.lines[e.Line+1:]...)
		return nil
	}
	// multiple lines across a multi-line range
	t.lines[s.Line] = t.lines[s.Line][:s.Character] + lines[0]
	t.lines[e.Line] = lines[len(lines)-1] + t.lines[e.Line][e.Character:]
	start := t.lines[:s.Line+1]
	end := t.lines[e.Line:]
	t.lines = append(append(start, lines[1:len(lines)-1]...), end...)
	return nil
}

func validateRange(r protocol.Range) error {
	if r.Start.Line > r.End.Line {
		return newInvalidRange(r, "start line %d > end line %d", r.Start.Line, r.End.Line)
	}
	if r.Start.Line == r.End.Line &&
		r.Start.Character > r.End.Character {
		return newInvalidRange(r, "start char %d > end char %d", r.Start.Character, r.End.Character)
	}
	return nil
}

func newInvalidRange(r protocol.Range, msg string, a ...interface{}) error {
	return invalidRange{r, fmt.Sprintf(msg, a...)}
}

type invalidRange struct {
	r      protocol.Range
	reason string
}

func (ir invalidRange) Error() string {
	return "Invalid range: " + ir.reason
}
