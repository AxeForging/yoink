# yoink

Background process manager with interactive I/O. Spawn commands in the background, check their output, send input to hanging processes, and reattach with full terminal control.

![yoink demo](docs/demo.gif)

## Install

```bash
# From source
go install github.com/AxeForging/yoink@latest

# From release
curl -Lo yoink.tar.gz https://github.com/AxeForging/yoink/releases/latest/download/yoink-linux-amd64.tar.gz
tar xzf yoink.tar.gz && sudo mv yoink /usr/local/bin/
```

## Usage

```bash
# Spawn a command in the background
yoink run "make deploy-staging"

# Spawn with an alias for easier reference
yoink run --alias deploy "make deploy-staging"

# List all managed processes
yoink ls

# Check output of a process (by ID or alias)
yoink log 1
yoink log deploy --lines 100

# Show only new lines since last --new call
yoink log deploy --new

# Render the current screen of any process — works correctly for TUI apps
# (yoink log gives raw escape garbage for TUIs; snapshot renders the actual frame)
yoink snapshot htop
yoink snap htop          # short alias

# Send input to a process waiting for confirmation
yoink send deploy "y"

# Send input character by character (simulates typing — great for asciinema recordings)
yoink send deploy "hello world" --type

# Control typing speed with --delay <ms> (default 50ms)
yoink send deploy "hello world" --type --delay 100

# Send without pressing Enter (useful for prompts that read a single key)
yoink send deploy "y" --no-enter

# Send a named key sequence (arrow keys, ctrl combos, function keys, etc.)
yoink send deploy --key up
yoink send deploy --key ctrl+c
yoink send deploy --key pagedown
yoink send deploy --key f5

# Send sensitive input (passwords, tokens) — hidden from logs
yoink send-redacted deploy "my-secret-token"

# Flags work on send-redacted too
yoink send-redacted deploy "my-secret" --type --delay 80

# Full interactive reattach (Ctrl+] to detach)
yoink attach deploy

# Wait for a process to finish (streams output, exits with process exit code)
yoink wait deploy --timeout 300 --poll 5

# Wait for multiple processes at once (fail-fast on first failure)
yoink wait build install runner-startup --timeout 300

# Wait with custom timeout message
yoink wait build --timeout 60 --message "Build exceeded 1min budget"

# Kill a process
yoink kill deploy
yoink kill 1 --force    # SIGKILL

# Remove finished processes
yoink clean
```

## How it works

yoink runs a lightweight daemon that holds PTY (pseudo-terminal) sessions open for each background process. The CLI client communicates with the daemon over a Unix socket.

- **run**: Spawns the command under a PTY in the daemon, returns an ID. Use `--alias` to name it
- **ls**: Lists all processes with state (running/done/failed), PID, exit code
- **log**: Shows buffered output (last 1000 lines kept per process). Use `--new` to see only lines since the last `--new` call
- **wait**: Polls a process until it finishes, streaming new output to stdout on each cycle. Exits 0 on success, propagates the process exit code on failure, or exits 124 on timeout (matches GNU `timeout` convention). Flags: `--timeout` (seconds, 0=forever), `--poll` (seconds, default 2), `--message` (custom timeout error)
- **snapshot** (`snap`): Renders the current VT100 screen of a process as plain text. Unlike `log`, this correctly handles TUI applications (htop, vim, Claude, etc.) by replaying their escape sequences through a VT100 emulator and returning only the rendered content. Works on both running and finished processes
- **send**: Writes text to the process stdin (fire-and-forget). Flags: `--type` (character by character), `--delay` (ms between chars, default 50), `--no-enter` (omit trailing newline), `--key` (send a named key sequence — e.g. `up`, `down`, `ctrl+c`, `f1`, `pageup`; implies `--no-enter`)
- **send-redacted**: Same as send, but input is masked in logs. Supports the same flags
- **attach**: Bridges your terminal to the process PTY for full interactive control
- **kill**: Sends SIGTERM (or SIGKILL with --force)
- **clean**: Removes finished processes from the list

The daemon auto-starts when needed and stores its socket at `~/.yoink/yoink.sock`.

## Platform Support

| Feature | Linux | macOS | Windows |
|---------|-------|-------|---------|
| run/ls/log/wait/send/send-redacted/kill/clean | Yes | Yes | Planned |
| send --type / --delay / --no-enter / --key | Yes | Yes | Planned |
| snapshot (VT100 screen render) | Yes | Yes | Planned |
| attach (PTY reattach) | Yes | Yes | Planned |

## Development

```bash
make build-local    # Build binary
make test           # Run tests
make lint           # Run linter
```

## License

MIT
