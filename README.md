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
- Hand the whole thing to an agent as native MCP tools with `yoink mcp`, sharing one daemon with your own terminal.

See [Why yoink?](docs/WHY.md) for the deeper positioning and comparisons.

## Install

```bash
# One-liner — auto-detects your OS/arch, installs the latest release
curl -fsSL https://raw.githubusercontent.com/AxeForging/yoink/main/install.sh | sh

# Pin a version, or install somewhere else
VERSION=v0.1.0 YOINK_INSTALL_DIR=~/.local/bin sh -c "$(curl -fsSL https://raw.githubusercontent.com/AxeForging/yoink/main/install.sh)"

# From source
go install github.com/AxeForging/yoink@latest
```

Prebuilt binaries for every OS/arch are attached to each [release](https://github.com/AxeForging/yoink/releases/latest).

### Use in CI (GitHub Actions)

Set up a pinned yoink on the runner, then overlap a slow background download with real work so the job blocks on it only when it has to:

```yaml
steps:
  - uses: actions/checkout@v4
  - uses: AxeForging/yoink@v0.1.0   # pin a release tag (use @main for latest)

  - run: npm ci

  # Playwright browsers are a slow download — start it in the background now.
  - name: Download browsers (background)
    run: yoink run --alias browsers "npx playwright install --with-deps chromium"

  # Real work runs while the download is in flight — this is where time is saved.
  - name: Build & typecheck (overlaps the download)
    run: |
      npm run build
      npm run typecheck

  # Block on the download only now, right before the tests need it.
  - name: Wait for browsers, then run e2e
    run: |
      yoink wait browsers --timeout 300 --poll 5
      npx playwright test
```

Wall-clock becomes `max(build + typecheck, browser download)` instead of the sum — the gain comes entirely from the work you do *between* `run` and `wait`. The action downloads the matching `yoink-<os>-<arch>` binary and adds it to `PATH`; Linux, macOS and Windows runners are supported.

### Use with AI agents (MCP)

The MCP server is built into the binary as `yoink mcp` — nothing extra to download. Register it once:

```bash
# Claude Code, available in every project
claude mcp add --scope user yoink -- yoink mcp

# or scoped to one repo (writes .mcp.json — commit it to share with your team)
claude mcp add --scope project yoink -- yoink mcp
```

For clients configured by file (`.mcp.json`, `.cursor/mcp.json`, Windsurf, Zed):

```json
{
  "mcpServers": {
    "yoink": { "command": "yoink", "args": ["mcp"] }
  }
}
```

VS Code uses `servers` instead of `mcpServers` in `.vscode/mcp.json`. GUI apps such as Claude Desktop do not inherit your shell `PATH`, so give those the absolute path (`/usr/local/bin/yoink`).

Agents get eight tools — `yoink_run`, `yoink_ls`, `yoink_log`, `yoink_send`, `yoink_snapshot`, `yoink_wait`, `yoink_kill`, `yoink_clean` — returning structured state instead of a text table. The server talks to the same daemon as the CLI, so an agent can inspect a dev server you started in your own terminal, and you can `yoink attach` to a process an agent started. See [the command reference](docs/COMMANDS.md#yoink-mcp) for details.

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
| Run in a specific directory | `yoink run --cwd ~/project "command"` |
| List managed processes | `yoink ls` |
| List with full command & cwd | `yoink ls --full` |
| Read recent output | `yoink log name --lines 100` |
| Read only new output | `yoink log name --new` |
| Send input | `yoink send name "text"` |
| Send a key | `yoink send name --key ctrl+c` |
| Send secret input | `yoink send-redacted name "$TOKEN"` |
| Render a TUI screen | `yoink snapshot name` |
| Wait with timeout | `yoink wait name --timeout 300` |
| Reattach interactively | `yoink attach name` |
| Stop a process | `yoink kill name` |
| Stop and remove everything | `yoink clean --all` |
| Serve to an AI agent over MCP | `yoink mcp` |

## Documentation

- [Why yoink?](docs/WHY.md): what problem it solves and how it compares to common alternatives.
- [Killer examples](docs/KILLER_EXAMPLES.md): compact real-world CI and automation cases where yoink removes painful glue code.
- [Command reference](docs/COMMANDS.md): commands, aliases, flags, and behavior notes.
- [Examples](EXAMPLES.md): real-world workflows for CI, prompts, TUIs, tunnels, AI agents, model servers, E2E tests, and more.
- [Architecture](docs/README.md): daemon/client design, PTY handling, snapshots, protocol, and platform notes.

## How It Works

yoink runs a lightweight daemon that holds PTY sessions open for background processes. The CLI talks to the daemon over a Unix socket at `~/.yoink/yoink.sock`, which auto-starts when needed.

## Platform Support

| Feature | Linux | macOS | Windows 10 1809+ |
|---------|-------|-------|---------|
| run/ls/log/wait/send/send-redacted/kill/clean | Yes | Yes | Yes |
| send --type / --delay / --no-enter / --key | Yes | Yes | Yes |
| snapshot (VT100 screen render) | Yes | Yes | Yes |
| attach (PTY reattach) | Yes | Yes | Yes |
| mcp (Model Context Protocol server) | Yes | Yes | Yes |

On Windows, commands run through `cmd.exe` (`%COMSPEC%`) inside a ConPTY, and `kill` stops the whole process tree (there is no SIGTERM, so it is always forceful). The socket lives at `%USERPROFILE%\.yoink\yoink.sock`. Use `go install` or the release `.zip`; `install.sh` is POSIX-only.

## Development

```bash
make build-local    # Build binary
make test           # Run tests
make lint           # Run linter
```

## License

MIT
