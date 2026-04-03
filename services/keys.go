package services

import (
	"fmt"
	"strings"
)

// keyMap maps human-readable key names to their PTY byte sequences.
var keyMap = map[string]string{
	// Arrow keys
	"up":    "\x1b[A",
	"down":  "\x1b[B",
	"right": "\x1b[C",
	"left":  "\x1b[D",

	// Navigation
	"home":     "\x1b[H",
	"end":      "\x1b[F",
	"pageup":   "\x1b[5~",
	"pgup":     "\x1b[5~",
	"pagedown": "\x1b[6~",
	"pgdn":     "\x1b[6~",
	"delete":   "\x1b[3~",
	"del":      "\x1b[3~",
	"insert":   "\x1b[2~",

	// Basic keys
	"enter":     "\r",
	"tab":       "\t",
	"backspace": "\x7f",
	"escape":    "\x1b",
	"esc":       "\x1b",

	// Ctrl combos
	"ctrl+a": "\x01",
	"ctrl+b": "\x02",
	"ctrl+c": "\x03",
	"ctrl+d": "\x04",
	"ctrl+e": "\x05",
	"ctrl+f": "\x06",
	"ctrl+k": "\x0b",
	"ctrl+l": "\x0c",
	"ctrl+n": "\x0e",
	"ctrl+p": "\x10",
	"ctrl+r": "\x12",
	"ctrl+u": "\x15",
	"ctrl+w": "\x17",
	"ctrl+z": "\x1a",
	"ctrl+]": "\x1d",

	// Function keys
	"f1":  "\x1bOP",
	"f2":  "\x1bOQ",
	"f3":  "\x1bOR",
	"f4":  "\x1bOS",
	"f5":  "\x1b[15~",
	"f6":  "\x1b[17~",
	"f7":  "\x1b[18~",
	"f8":  "\x1b[19~",
	"f9":  "\x1b[20~",
	"f10": "\x1b[21~",
	"f11": "\x1b[23~",
	"f12": "\x1b[24~",
}

// ResolveKey returns the PTY byte sequence for the given key name.
// Names are case-insensitive (e.g. "Up", "CTRL+C", "PageUp").
func ResolveKey(name string) (string, error) {
	seq, ok := keyMap[strings.ToLower(name)]
	if !ok {
		return "", fmt.Errorf("unknown key %q — run 'yoink send --help' for supported key names", name)
	}
	return seq, nil
}
