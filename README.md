# yoink

Background process manager with interactive I/O. Spawn commands in the background, check their output, send input to hanging processes, and reattach with full terminal control.

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

# Send input to a process waiting for confirmation
yoink send deploy "y"

# Send input character by character (simulates typing — great for asciinema recordings)
yoink send deploy "hello world" --type

# Control typing speed with --delay <ms> (default 50ms)
yoink send deploy "hello world" --type --delay 100

# Send without pressing Enter (useful for prompts that read a single key)
yoink send deploy "y" --no-enter

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
- **send**: Writes text to the process stdin (fire-and-forget). Flags: `--type` (character by character), `--delay` (ms between chars, default 50), `--no-enter` (omit trailing newline)
- **send-redacted**: Same as send, but input is masked in logs. Supports the same `--type`/`--delay`/`--no-enter` flags
- **attach**: Bridges your terminal to the process PTY for full interactive control
- **kill**: Sends SIGTERM (or SIGKILL with --force)
- **clean**: Removes finished processes from the list

The daemon auto-starts when needed and stores its socket at `~/.yoink/yoink.sock`.

## Platform Support

| Feature | Linux | macOS | Windows |
|---------|-------|-------|---------|
| run/ls/log/wait/send/send-redacted/kill/clean | Yes | Yes | Planned |
| send --type / --delay / --no-enter | Yes | Yes | Planned |
| attach (PTY reattach) | Yes | Yes | Planned |

## Development

```bash
make build-local    # Build binary
make test           # Run tests
make lint           # Run linter
```

## License

MIT
