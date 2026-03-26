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

# List all managed processes
yoink ls

# Check output of a process
yoink log 1
yoink log 1 --lines 100

# Send input to a process waiting for confirmation
yoink send 1 "y"

# Full interactive reattach (Ctrl+] to detach)
yoink attach 1

# Kill a process
yoink kill 1
yoink kill 1 --force    # SIGKILL

# Remove finished processes
yoink clean
```

## How it works

yoink runs a lightweight daemon that holds PTY (pseudo-terminal) sessions open for each background process. The CLI client communicates with the daemon over a Unix socket.

- **run**: Spawns the command under a PTY in the daemon, returns an ID
- **ls**: Lists all processes with state (running/done/failed), PID, exit code
- **log**: Shows buffered output (last 1000 lines kept per process)
- **send**: Writes text to the process stdin (fire-and-forget)
- **attach**: Bridges your terminal to the process PTY for full interactive control
- **kill**: Sends SIGTERM (or SIGKILL with --force)
- **clean**: Removes finished processes from the list

The daemon auto-starts when needed and stores its socket at `~/.yoink/yoink.sock`.

## Platform Support

| Feature | Linux | macOS | Windows |
|---------|-------|-------|---------|
| run/ls/log/send/kill/clean | Yes | Yes | Planned |
| attach (PTY reattach) | Yes | Yes | Planned |

## Development

```bash
make build-local    # Build binary
make test           # Run tests
make lint           # Run linter
```

## License

MIT
