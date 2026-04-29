package chat

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"

	"github.com/papermap/papermap-tui/internal/theme"
)

// Pasted text that meets either threshold below collapses into a chip
// instead of being inserted verbatim into the textarea. Chosen to match
// common chat-input behavior: short snippets stay editable, large blobs
// become an opaque reference the user can drop with one backspace.
const (
	pasteMinLines = 4
	pasteMinChars = 400
)

// pasteToken is the in-buffer marker stored inside the textarea value
// for each collapsed paste. The renderer rewrites every match into a
// styled "[Pasted ~N lines]" chip; the submit path expands matches back
// to the original content. Deliberately short and ASCII so cursor drift
// from token-vs-chip width difference stays small.
//
// Only tokens that resolve to a registered chip are decorated, so a user
// who types this string literally will see it pass through unchanged.
var pasteTokenRE = regexp.MustCompile(`\[#p:(\d+)\]`)

// pasteChip is a single collapsed paste tracked by pasteRegistry.
type pasteChip struct {
	id      int
	content string
	lines   int
}

// pasteRegistry stores every collapsed paste for the current prompt
// draft. Reset whenever the textarea is cleared (submit, ctrl+l,
// LoadConversation).
type pasteRegistry struct {
	chips  map[int]pasteChip
	nextID int
}

func newPasteRegistry() pasteRegistry {
	return pasteRegistry{chips: map[int]pasteChip{}, nextID: 1}
}

// shouldCollapsePaste reports whether a pasted blob exceeds either the
// line or character threshold.
func shouldCollapsePaste(s string) bool {
	if len(s) >= pasteMinChars {
		return true
	}
	// Count newlines + 1 so a single-line paste counts as 1 line.
	if strings.Count(s, "\n")+1 >= pasteMinLines {
		return true
	}
	return false
}

// add stores text and returns the in-buffer token to insert at the
// cursor. The caller is responsible for inserting the token into the
// textarea.
func (r *pasteRegistry) add(text string) (token string, lines int) {
	id := r.nextID
	r.nextID++
	lines = strings.Count(text, "\n") + 1
	r.chips[id] = pasteChip{id: id, content: text, lines: lines}
	return fmt.Sprintf("[#p:%d]", id), lines
}

// reset drops every chip. Called on submit, clear, and conversation
// load so chips never bleed across prompts.
func (r *pasteRegistry) reset() {
	r.chips = map[int]pasteChip{}
	r.nextID = 1
}

// remove deletes a single chip by id. Used after a backspace-on-chip
// removes the in-buffer token so the chip cannot leak into the next
// submit if the user re-pastes.
func (r *pasteRegistry) remove(id int) {
	delete(r.chips, id)
}

// expand replaces every registered token in s with its original text.
// Unknown tokens (no matching chip) are left as-is so a user-typed
// literal like "[#p:99]" stays intact.
func (r *pasteRegistry) expand(s string) string {
	if len(r.chips) == 0 {
		return s
	}
	return pasteTokenRE.ReplaceAllStringFunc(s, func(match string) string {
		id, ok := parsePasteID(match)
		if !ok {
			return match
		}
		chip, ok := r.chips[id]
		if !ok {
			return match
		}
		return chip.content
	})
}

// chipBeforeCursor returns the chip whose token ends exactly at the
// given byte offset in the raw textarea value, plus the byte range the
// token occupies. Used by the backspace handler to delete a chip in one
// keystroke when the cursor sits immediately after it.
//
// Returns ok=false when no chip token ends at the offset.
func (r *pasteRegistry) chipBeforeCursor(value string, byteOffset int) (chip pasteChip, start, end int, ok bool) {
	if byteOffset <= 0 || len(r.chips) == 0 {
		return pasteChip{}, 0, 0, false
	}
	if byteOffset > len(value) {
		byteOffset = len(value)
	}
	matches := pasteTokenRE.FindAllStringIndex(value, -1)
	for _, m := range matches {
		if m[1] != byteOffset {
			continue
		}
		id, idOK := parsePasteID(value[m[0]:m[1]])
		if !idOK {
			continue
		}
		c, exists := r.chips[id]
		if !exists {
			continue
		}
		return c, m[0], m[1], true
	}
	return pasteChip{}, 0, 0, false
}

// decorate rewrites every registered paste token in rendered with the
// styled chip label. rendered is typically the output of
// textarea.View(); chip labels inherit the textarea's input background
// so they sit cleanly inside the prompt box.
func (r *pasteRegistry) decorate(rendered string, th theme.Theme) string {
	if len(r.chips) == 0 {
		return rendered
	}
	return pasteTokenRE.ReplaceAllStringFunc(rendered, func(match string) string {
		id, ok := parsePasteID(match)
		if !ok {
			return match
		}
		chip, exists := r.chips[id]
		if !exists {
			return match
		}
		return chipLabel(th, chip.lines)
	})
}

// chipLabel renders the visible "[Pasted ~N lines]" pill. Styled to
// stand out against the input background while still reading as inline
// content, mirroring the screenshot the spec was based on.
func chipLabel(th theme.Theme, lines int) string {
	style := lipgloss.NewStyle().
		Foreground(th.InputBg).
		Background(th.LogoColorB).
		Bold(true)
	return style.Render(fmt.Sprintf("[Pasted ~%d lines]", lines))
}

// parsePasteID extracts the numeric id from a "[#p:N]" token.
func parsePasteID(token string) (int, bool) {
	m := pasteTokenRE.FindStringSubmatch(token)
	if len(m) != 2 {
		return 0, false
	}
	var id int
	for _, r := range m[1] {
		if r < '0' || r > '9' {
			return 0, false
		}
		id = id*10 + int(r-'0')
	}
	return id, true
}

// runeIndexToByteOffset converts a rune-based cursor position in s to
// the corresponding byte offset, clamped to the bounds of s. Used to
// translate the textarea cursor (column index in the current line) into
// the absolute byte offset needed for chipBeforeCursor.
func runeIndexToByteOffset(s string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	count := 0
	for i := range s {
		if count == runeIdx {
			return i
		}
		count++
	}
	if count <= runeIdx {
		return len(s)
	}
	return len(s)
}

// utf8Len returns the rune count of s. Wrapper kept tiny so call sites
// read clearly when converting between rune offsets and byte offsets.
func utf8Len(s string) int {
	return utf8.RuneCountInString(s)
}

// cursorByteOffset converts the textarea's (line, column) cursor
// position into the absolute byte offset within value. Lines are
// joined by '\n'. Column is interpreted as a rune offset within the
// current logical line, matching textarea.Column() semantics.
func cursorByteOffset(value string, line, column int) int {
	if line < 0 {
		line = 0
	}
	if column < 0 {
		column = 0
	}
	lines := strings.Split(value, "\n")
	if line >= len(lines) {
		return len(value)
	}
	offset := 0
	for i := 0; i < line; i++ {
		offset += len(lines[i]) + 1 // +1 for '\n'
	}
	offset += runeIndexToByteOffset(lines[line], column)
	if offset > len(value) {
		offset = len(value)
	}
	return offset
}

// placeCursorAtRuneOffset positions the textarea cursor at the rune
// index runeOffset within value. Walks from the beginning so it works
// regardless of where SetValue left the cursor.
func placeCursorAtRuneOffset(ta *textarea.Model, value string, runeOffset int) {
	if runeOffset < 0 {
		runeOffset = 0
	}
	// Determine the logical (line, column) for the rune offset.
	lines := strings.Split(value, "\n")
	targetLine := 0
	targetCol := 0
	remaining := runeOffset
	for i, l := range lines {
		runeLen := utf8.RuneCountInString(l)
		if remaining <= runeLen {
			targetLine = i
			targetCol = remaining
			break
		}
		// +1 accounts for the '\n' separator between lines.
		remaining -= runeLen + 1
		if remaining < 0 {
			targetLine = i
			targetCol = runeLen
			break
		}
	}

	// Walk the cursor to the target line: jump to the top, then step
	// down. The textarea has no public API for absolute row movement,
	// so this is the cleanest portable approach.
	ta.MoveToBegin()
	for i := 0; i < targetLine; i++ {
		ta.CursorDown()
	}
	ta.SetCursorColumn(targetCol)
}
