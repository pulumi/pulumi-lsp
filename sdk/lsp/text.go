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

func (d *Document) URI() protocol.DocumentURI {
	return d.uri
}

// Returns the whole document as a string.
func (d *Document) String() string {
	d.m.RLock()
	defer d.m.RUnlock()
	return strings.Join(d.lines, lineDeliminator)
}

// Window provides the text of the document that fits in the window.
func (d *Document) Window(window protocol.Range) (string, error) {
	// This is only the range, which was passed by value
	if err := validateRange(window); err != nil {
		return "", err
	}
	d.m.RLock()
	defer d.m.RUnlock()
	if err := d.validateRange(window); err != nil {
		return "", err
	}
	sLine := int(window.Start.Line)
	sChar := int(window.Start.Character)
	eLine := int(window.End.Line)
	eChar := int(window.End.Character)
	if window.Start.Line == window.End.Line {
		return d.lines[sLine][sChar:eChar], nil
	}
	return d.lines[sLine][sChar:] + strings.Join(d.lines[sLine+1:eLine], lineDeliminator) + d.lines[eLine][:eChar], nil
}

func (d *Document) Line(i int) (protocol.Range, error) {
	if i <= 0 {
		return protocol.Range{}, fmt.Errorf("Cannot access negative line")
	}
	if i >= len(d.lines) {
		return protocol.Range{}, fmt.Errorf("Line index is %d but there are only %d lines", i, len(d.lines))
	}
	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(i),
			Character: 0,
		},
		End: protocol.Position{
			Line:      uint32(i),
			Character: uint32(len(d.lines[i])),
		},
	}, nil
}

// Validate that the range is in the Text. Calling validateRange requires
// holding any lock on the document.
func (d *Document) validateRange(r protocol.Range) error {
	sLine := int(r.Start.Line)
	sChar := int(r.Start.Character)
	if sLine >= len(d.lines) {
		return newInvalidRange(r, "start line %d out of bounds for document with %d lines", sLine, len(d.lines))
	}
	if sChar >= len(d.lines[sLine]) {
		return newInvalidRange(r, "start character %d out of bound on line %d", sChar, sLine)
	}
	eLine := int(r.End.Line)
	eChar := int(r.End.Character)
	if eLine >= len(d.lines) {
		return newInvalidRange(r, "end line %d out of bounds for document with %d lines", eLine, len(d.lines))
	}
	if eChar > len(d.lines[eLine]) {
		return newInvalidRange(r, "end character %d out of bound on line %d (len = %d)", eChar, eLine, len(d.lines[eLine]))
	}
	return nil
}

// acceptChange implements the change on the document. Calling acceptChange
// correctly requires holding a write lock on the document.
func (d *Document) acceptChange(change protocol.TextDocumentContentChangeEvent) error {
	var defRange protocol.Range
	if change.Range == defRange && change.RangeLength == 0 {
		// This indicates that the whole document should be changed.
		d.lines = strings.Split(change.Text, lineDeliminator)
		return nil
	}
	// Note: RangeLength is depreciated
	lines := strings.Split(change.Text, lineDeliminator)
	s := change.Range.Start
	e := change.Range.End
	contract.Assert(len(lines) != 0)

	if s.Line == e.Line {
		l := d.lines[s.Line]
		if len(lines) == 1 {
			// We are replacing withing a line
			if int(s.Character) > len(l) {
				panic(fmt.Sprintf("s.Char{%d} > len(l){%d}: %#v:\n\n%#v\n%#v\n%#v",
					int(s.Character), len(l), change, d.lines[s.Line-1], d.lines[s.Line], d.lines[s.Line+1]))

			}
			d.lines[s.Line] = l[:s.Character] + change.Text + l[e.Character:]
			return nil
		}
		// We need to add new lines
		end := l[e.Character:]
		start := l[:s.Character] + lines[0]
		lines[len(lines)-1] += end
		d.lines = append(append(append(d.lines[:s.Line], start), lines...), d.lines[e.Line:]...)
		return nil
	}
	// Range is across multiple lines
	if len(lines) == 1 {
		// Joining lines together
		d.lines[s.Line] = d.lines[s.Line][:s.Character] + lines[0] + d.lines[e.Line][e.Character:]
		d.lines = append(d.lines[:s.Line+1], d.lines[e.Line+1:]...)
		return nil
	}
	// multiple lines across a multi-line range
	d.lines[s.Line] = d.lines[s.Line][:s.Character] + lines[0]
	d.lines[e.Line] = lines[len(lines)-1] + d.lines[e.Line][e.Character:]
	start := d.lines[:s.Line+1]
	end := d.lines[e.Line:]
	d.lines = append(append(start, lines[1:len(lines)-1]...), end...)
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
