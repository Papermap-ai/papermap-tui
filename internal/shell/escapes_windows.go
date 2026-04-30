//go:build windows

package shell

// escapeIntroducers on Windows recognizes ESC only. The 8-bit C1
// introducers (0x90, 0x98, 0x9b, 0x9d, 0x9e, 0x9f) collide with
// UTF-8 continuation bytes; including them would split multibyte
// runes mid-character when the Windows console emits UTF-8 output.
// ESC-introduced sequences are still scrubbed via the standard
// 7-bit path in sanitizeANSI.
var escapeIntroducers = []byte{0x1b}
