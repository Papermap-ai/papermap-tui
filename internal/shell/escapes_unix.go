//go:build unix

package shell

// escapeIntroducers on Unix recognizes ESC plus the 8-bit C1
// introducers (CSI, OSC, DCS, PM, APC, SOS). Unix terminals are
// commonly UTF-8 but the C1 range is rarely seen in legitimate
// program output, and the cost of an occasional false positive
// inside a non-ASCII rune is preferable to letting an OSC 52
// clipboard write through.
var escapeIntroducers = []byte{0x1b, 0x9b, 0x9d, 0x90, 0x9e, 0x9f, 0x98}
