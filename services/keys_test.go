package services

import "testing"

func TestResolveKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Arrow keys
		{name: "up", input: "up", want: "\x1b[A"},
		{name: "down", input: "down", want: "\x1b[B"},
		{name: "right", input: "right", want: "\x1b[C"},
		{name: "left", input: "left", want: "\x1b[D"},

		// Navigation
		{name: "home", input: "home", want: "\x1b[H"},
		{name: "end", input: "end", want: "\x1b[F"},
		{name: "pageup", input: "pageup", want: "\x1b[5~"},
		{name: "pgup alias", input: "pgup", want: "\x1b[5~"},
		{name: "pagedown", input: "pagedown", want: "\x1b[6~"},
		{name: "pgdn alias", input: "pgdn", want: "\x1b[6~"},
		{name: "delete", input: "delete", want: "\x1b[3~"},
		{name: "del alias", input: "del", want: "\x1b[3~"},
		{name: "insert", input: "insert", want: "\x1b[2~"},

		// Basic keys
		{name: "enter", input: "enter", want: "\r"},
		{name: "tab", input: "tab", want: "\t"},
		{name: "backspace", input: "backspace", want: "\x7f"},
		{name: "escape", input: "escape", want: "\x1b"},
		{name: "esc alias", input: "esc", want: "\x1b"},

		// Ctrl combos
		{name: "ctrl+c", input: "ctrl+c", want: "\x03"},
		{name: "ctrl+d", input: "ctrl+d", want: "\x04"},
		{name: "ctrl+z", input: "ctrl+z", want: "\x1a"},
		{name: "ctrl+l", input: "ctrl+l", want: "\x0c"},
		{name: "ctrl+r", input: "ctrl+r", want: "\x12"},
		{name: "ctrl+u", input: "ctrl+u", want: "\x15"},
		{name: "ctrl+]", input: "ctrl+]", want: "\x1d"},

		// Function keys
		{name: "f1", input: "f1", want: "\x1bOP"},
		{name: "f4", input: "f4", want: "\x1bOS"},
		{name: "f5", input: "f5", want: "\x1b[15~"},
		{name: "f10", input: "f10", want: "\x1b[21~"},
		{name: "f12", input: "f12", want: "\x1b[24~"},

		// Case insensitive
		{name: "Up uppercase", input: "Up", want: "\x1b[A"},
		{name: "CTRL+C uppercase", input: "CTRL+C", want: "\x03"},
		{name: "PageUp mixed case", input: "PageUp", want: "\x1b[5~"},
		{name: "F1 uppercase", input: "F1", want: "\x1bOP"},

		// Unknown key
		{name: "unknown key", input: "superkey", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveKey(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveKey(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveKey(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ResolveKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
