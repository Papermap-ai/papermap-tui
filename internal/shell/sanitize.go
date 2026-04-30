package shell

import "strings"

// sanitizeANSI strips ANSI escape sequences from raw command output
// before it enters the chat transcript. The goal is defensive: a
// malicious or careless command can emit OSC 52 (clipboard write),
// OSC 8 (hyperlinks pointing anywhere), DCS / APC / PM / SOS payloads,
// or device-attribute responses that some terminals reply to by
// writing to stdin. None of those make sense inside a chat bubble,
// and the safe response is to drop them all.
//
// v1 strips every escape sequence including SGR color codes. A future
// pass can selectively re-admit SGR so colored output (git, ls --color)
// renders. Until then, plain text is the right tradeoff.
func sanitizeANSI(s string) string {
	if !containsAnyByte(s, escapeIntroducers) {
		// Fast path: no escape, no control-string introducers.
		return stripBareControls(s)
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		switch c {
		case 0x1b: // ESC
			adv := skipEscape(s, i)
			if adv > 0 {
				i += adv
				continue
			}
			i++
		case 0x9b, 0x9d, 0x90, 0x9e, 0x9f: // CSI/OSC/DCS/PM/APC (8-bit)
			i = skipUntilST(s, i+1)
		case 0x98: // SOS
			i = skipUntilST(s, i+1)
		default:
			if isPrintableOrAllowedControl(c) {
				b.WriteByte(c)
			}
			i++
		}
	}
	return b.String()
}

// skipEscape consumes an ESC-introduced sequence starting at s[i]
// (s[i] == 0x1b) and returns the number of bytes consumed. Returns 0
// when no recognizable sequence is present so the caller can drop the
// stray ESC and continue.
func skipEscape(s string, i int) int {
	if i+1 >= len(s) {
		return 1
	}
	next := s[i+1]
	switch next {
	case '[': // CSI: ESC [ params... final
		j := i + 2
		for j < len(s) {
			c := s[j]
			if c >= 0x40 && c <= 0x7e {
				return j - i + 1
			}
			j++
		}
		return len(s) - i
	case ']': // OSC: ESC ] ... ST or BEL
		end := scanString(s, i+2)
		return end - i
	case 'P', '^', '_', 'X': // DCS / PM / APC / SOS
		end := scanString(s, i+2)
		return end - i
	default:
		// 2-byte escape (e.g. ESC =, ESC >, charset selectors).
		return 2
	}
}

// scanString consumes a control string starting at s[start] until it
// finds ST (ESC \) or BEL (0x07). Returns the index after the
// terminator, or len(s) if the stream ends mid-string.
func scanString(s string, start int) int {
	i := start
	for i < len(s) {
		c := s[i]
		if c == 0x07 { // BEL
			return i + 1
		}
		if c == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}
		if c == 0x9c { // 8-bit ST
			return i + 1
		}
		i++
	}
	return len(s)
}

// skipUntilST is the 8-bit-introducer counterpart to scanString.
func skipUntilST(s string, start int) int {
	return scanString(s, start)
}

// stripBareControls removes lone C0/C1 control bytes that are not part
// of an escape sequence. Tabs, newlines, and carriage returns survive
// because they are meaningful inside transcript output.
func stripBareControls(s string) string {
	if !hasBareControl(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if isPrintableOrAllowedControl(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func hasBareControl(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isPrintableOrAllowedControl(s[i]) {
			return true
		}
	}
	return false
}

func isPrintableOrAllowedControl(c byte) bool {
	switch c {
	case '\t', '\n', '\r':
		return true
	}
	return c >= 0x20 && c != 0x7f
}

// escapeIntroducers is the set of single-byte values that begin a
// terminal escape sequence (ESC plus the 8-bit C1 introducers we
// care about: CSI, OSC, DCS, PM, APC, SOS).
var escapeIntroducers = []byte{0x1b, 0x9b, 0x9d, 0x90, 0x9e, 0x9f, 0x98}

func containsAnyByte(s string, set []byte) bool {
	for i := 0; i < len(s); i++ {
		for _, b := range set {
			if s[i] == b {
				return true
			}
		}
	}
	return false
}
