package actions

import (
	"flag"
	"testing"

	"github.com/urfave/cli"
)

func newSendContext(t *testing.T, args []string, key string, noEnter bool) *cli.Context {
	t.Helper()
	app := cli.NewApp()
	set := flag.NewFlagSet("send", flag.ContinueOnError)
	set.String("key", "", "")
	set.Bool("no-enter", false, "")
	if err := set.Parse(args); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
	if key != "" {
		if err := set.Set("key", key); err != nil {
			t.Fatalf("set key flag: %v", err)
		}
	}
	if noEnter {
		if err := set.Set("no-enter", "true"); err != nil {
			t.Fatalf("set no-enter flag: %v", err)
		}
	}
	return cli.NewContext(app, set, nil)
}

func TestResolveInput(t *testing.T) {
	tests := []struct {
		name        string
		args        []string // positional args (alias + optional text)
		key         string
		noEnter     bool
		wantInput   string
		wantNoEnter bool
		wantErr     bool
	}{
		// Plain text: returns text, respects --no-enter
		{
			name:        "plain text without no-enter",
			args:        []string{"myproc", "hello"},
			wantInput:   "hello",
			wantNoEnter: false,
		},
		{
			name:        "plain text with no-enter",
			args:        []string{"myproc", "hello"},
			noEnter:     true,
			wantInput:   "hello",
			wantNoEnter: true,
		},
		{
			name:        "multi-word text joined with space",
			args:        []string{"myproc", "hello", "world"},
			wantInput:   "hello world",
			wantNoEnter: false,
		},
		{
			name:        "no text args returns empty string",
			args:        []string{"myproc"},
			wantInput:   "",
			wantNoEnter: false,
		},

		// --key mode: resolves sequence, always implies no-enter
		{
			name:        "--key up resolves to escape sequence",
			args:        []string{"myproc"},
			key:         "up",
			wantInput:   "\x1b[A",
			wantNoEnter: true,
		},
		{
			name:        "--key ctrl+c resolves to 0x03",
			args:        []string{"myproc"},
			key:         "ctrl+c",
			wantInput:   "\x03",
			wantNoEnter: true,
		},
		{
			name:        "--key is case-insensitive",
			args:        []string{"myproc"},
			key:         "PageUp",
			wantInput:   "\x1b[5~",
			wantNoEnter: true,
		},

		// Error: --key + text are mutually exclusive
		{
			name:    "--key and text are mutually exclusive",
			args:    []string{"myproc", "sometext"},
			key:     "up",
			wantErr: true,
		},

		// Error: unknown key
		{
			name:    "unknown key name returns error",
			args:    []string{"myproc"},
			key:     "superkey",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newSendContext(t, tt.args, tt.key, tt.noEnter)
			input, noEnter, err := resolveInput(ctx)
			if tt.wantErr {
				if err == nil {
					t.Errorf("resolveInput() expected error, got input=%q noEnter=%v", input, noEnter)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveInput() unexpected error: %v", err)
			}
			if input != tt.wantInput {
				t.Errorf("input = %q, want %q", input, tt.wantInput)
			}
			if noEnter != tt.wantNoEnter {
				t.Errorf("noEnter = %v, want %v", noEnter, tt.wantNoEnter)
			}
		})
	}
}
