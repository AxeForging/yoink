<p align="center">
  <img src="docs/assets/yoink-logo.png" alt="yoink logo" width="180">
</p>

<h1 align="center">yoink</h1>

Run interactive commands in the background, inspect their output, send input, capture TUI screens, wait on them from scripts, and reattach when you need full control.

`yoink` is a lightweight PTY process manager for developers, CI jobs, automation scripts, and AI-agent workflows. It fills the space between shell background jobs, `tmux`, `expect`, and full process supervisors: every process gets a real terminal, a name, logs, input control, lifecycle state, and a way back in.

![yoink demo](docs/demo.gif)

## Why yoink?

- Keep long-running commands alive without dedicating a terminal tab.
- Drive interactive CLIs from scripts with `send`, `send --key`, and `send-redacted`.
- Read only new output with `log --new`, which is useful for polling and test reports.
- Render TUI apps with `snapshot` instead of raw escape-sequence logs.
- Coordinate dev servers, test runners, deploys, tunnels, model servers, and AI coding agents.

See [Why yoink?](docs/WHY.md) for the deeper positioning and comparisons.

## Install

```bash
# From source
go install github.com/AxeForging/yoink@latest

# From release
curl -Lo yoink.tar.gz https://github.com/AxeForging/yoink/releases/latest/download/yoink-linux-amd64.tar.gz
tar xzf yoink.tar.gz && sudo mv yoink /usr/local/bin/
```

## Quickstart

```bash
# Start a command under a managed PTY
yoink run --alias deploy "make deploy-staging"

# Check state and output
yoink ls
yoink log deploy --new

# Answer a prompt without reattaching
yoink send deploy "y"

# Wait from a script, streaming new output
yoink wait deploy --timeout 300 --poll 5

# Reattach later, or clean up
yoink attach deploy      # Ctrl+] detaches
yoink kill deploy
yoink clean
```

## Common Commands

| Task | Command |
|------|---------|
| Run in background | `yoink run --alias name "command"` |
| List managed processes | `yoink ls` |
| Read recent output | `yoink log name --lines 100` |
| Read only new output | `yoink log name --new` |
| Send input | `yoink send name "text"` |
| Send a key | `yoink send name --key ctrl+c` |
| Send secret input | `yoink send-redacted name "$TOKEN"` |
| Render a TUI screen | `yoink snapshot name` |
| Wait with timeout | `yoink wait name --timeout 300` |
| Reattach interactively | `yoink attach name` |
| Stop a process | `yoink kill name` |

## Documentation

- [Why yoink?](docs/WHY.md): what problem it solves and how it compares to common alternatives.
- [Killer examples](docs/KILLER_EXAMPLES.md): compact real-world CI and automation cases where yoink removes painful glue code.
- [Command reference](docs/COMMANDS.md): commands, aliases, flags, and behavior notes.
- [Examples](EXAMPLES.md): real-world workflows for CI, prompts, TUIs, tunnels, AI agents, model servers, E2E tests, and more.
- [Architecture](docs/README.md): daemon/client design, PTY handling, snapshots, protocol, and platform notes.

## How It Works

yoink runs a lightweight daemon that holds PTY sessions open for background processes. The CLI talks to the daemon over a Unix socket at `~/.yoink/yoink.sock`, which auto-starts when needed.

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
