# Command Reference

This page covers the command surface. For longer workflows, see [Examples](../EXAMPLES.md).

## Process Lifecycle

### `yoink run`

Spawn a command under a managed PTY.

```bash
yoink run "make deploy-staging"
yoink run --alias deploy "make deploy-staging"
yoink run --cwd ~/projects/api "npm test"
yoink run --log-file /tmp/build-full.log "make build"
```

Aliases: `r`

Flags:

| Flag | Description |
|------|-------------|
| `--alias <name>` | Assign a stable name for later commands |
| `--cwd <dir>` | Working directory for the process. Default: your current directory. Relative paths resolve against it |
| `--log-file <path>` | Also tee full raw output to this file, unbounded by the in-memory log buffer |

`yoink` runs as a long-lived daemon, so without `--cwd` it always uses *your* current directory at the moment you run `yoink run` — not wherever the daemon happened to start.

`--log-file` is useful when a process produces more output than the default in-memory buffer keeps (e.g. a noisy build): `log`/`log --new` only ever see the buffered tail, but the file on disk has everything.

### `yoink ls`

List managed processes with state, PID, aliases, and exit information.

```bash
yoink ls
yoink ls --full
```

Aliases: `list`, `ps`

Flags:

| Flag | Description |
|------|-------------|
| `--full` | Show the untruncated command and a CWD column, instead of a 60-char-truncated command |

### `yoink kill`

Terminate a managed process.

```bash
yoink kill deploy
yoink kill deploy --force
```

Aliases: `k`

Flags:

| Flag | Description |
|------|-------------|
| `--force`, `-f` | Send SIGKILL instead of SIGTERM |

### `yoink clean`

Remove finished or failed processes from the process list.

```bash
yoink clean
yoink clean --all
yoink clean --all --force
```

Flags:

| Flag | Description |
|------|-------------|
| `--all`, `-a` | Also stop still-running processes (SIGTERM, or SIGKILL with `--force`) before removing everything |
| `--force`, `-f` | With `--all`, send SIGKILL instead of SIGTERM |

Plain `clean` only ever touches finished/failed processes. `clean --all` is the one-shot "start over" command — it stops whatever is still running first, then removes everything.

## Output

### `yoink log`

Show buffered output from a process. The daemon keeps the recent output for each managed process.

```bash
yoink log deploy
yoink log deploy --lines 100
yoink log deploy --new
```

Aliases: `l`

Flags:

| Flag | Description |
|------|-------------|
| `--lines`, `-n` | Number of output lines to show. Default: `50` |
| `--new` | Show only lines not yet seen by a previous `--new` call |

Use `log` for normal line-oriented programs. Use `snapshot` for TUIs.

### `yoink snapshot`

Render the current VT100 screen of a process as plain text.

```bash
yoink snapshot htop
yoink snap htop
```

Aliases: `snap`

This is useful for TUI apps such as `htop`, `vim`, `k9s`, chat CLIs, and AI coding agents. `log` shows raw terminal bytes for those programs; `snapshot` shows the rendered screen.

## Input

### `yoink send`

Send text or key sequences to a running process.

```bash
yoink send deploy "y"
yoink send repl "hello world" --type --delay 80
yoink send claude "say only OK" --submit
yoink send tui --key down
yoink send tui --key ctrl+c
```

Aliases: `s`

Flags:

| Flag | Description |
|------|-------------|
| `--type` | Send input character by character |
| `--delay`, `-d` | Milliseconds between characters with `--type`. Default: `50` |
| `--no-enter` | Do not send a trailing newline |
| `--submit` | Submit text with carriage-return Enter, useful for TUIs |
| `--key`, `-k` | Send a named key sequence instead of text |

Common keys include `up`, `down`, `left`, `right`, `enter`, `tab`, `escape`, `ctrl+c`, `f1`, and `pageup`.

### `yoink send-redacted`

Send sensitive input while masking it in yoink logs.

```bash
yoink send-redacted deploy "$DB_PASSWORD"
yoink send-redacted deploy "$TOKEN" --type --delay 80
```

Flags:

| Flag | Description |
|------|-------------|
| `--type` | Send input character by character |
| `--delay`, `-d` | Milliseconds between characters with `--type`. Default: `50` |
| `--no-enter` | Do not send a trailing newline |
| `--submit` | Submit text with carriage-return Enter, useful for TUIs |

## Waiting And Reattaching

### `yoink wait`

Wait for one or more processes to finish. Output is streamed while waiting.

```bash
yoink wait deploy --timeout 300 --poll 5
yoink wait build test lint --timeout 600
yoink wait migrate --timeout 600 --message "Migration exceeded 10min budget"
```

Aliases: `w`

Flags:

| Flag | Description |
|------|-------------|
| `--timeout`, `-t` | Maximum seconds to wait. `0` means forever |
| `--poll`, `-p` | Seconds between state checks. Default: `2` |
| `--message`, `-m` | Custom error message on timeout |

Exit behavior:

| Result | Exit code |
|--------|-----------|
| All processes succeeded | `0` |
| A process failed | That process exit code |
| Timeout | `124` |

### `yoink attach`

Bridge your terminal to the process PTY for full interactive control.

```bash
yoink attach deploy
```

Aliases: `a`

Detach with `Ctrl+]`.

## AI Agents

### `yoink mcp`

Serve yoink over the [Model Context Protocol](https://modelcontextprotocol.io) on stdio, so an agent manages background processes as native tools instead of shelling out and parsing terminal output.

```bash
yoink mcp
```

You do not run this yourself — your MCP client does. Register it once:

```bash
claude mcp add --scope user yoink -- yoink mcp
```

The server is a client of the same daemon the CLI uses, so an agent's `yoink_ls` sees processes you started in your own terminal, and vice versa.

Tools:

| Tool | Equivalent command |
|------|--------------------|
| `yoink_run` | `yoink run` |
| `yoink_ls` | `yoink ls --full` |
| `yoink_log` | `yoink log` (`new: true` for `--new`) |
| `yoink_send` | `yoink send` / `send-redacted` (`redacted: true`) |
| `yoink_snapshot` | `yoink snapshot` |
| `yoink_wait` | `yoink wait` |
| `yoink_kill` | `yoink kill` |
| `yoink_clean` | `yoink clean` |

`yoink attach` is not exposed: it needs a real controlling terminal.

`yoink_wait` differs from the CLI in one way. MCP clients cap how long a single tool call may run, so its `timeoutSeconds` defaults to `30` and is clamped to `55`. Hitting that bound is **not an error** — the call returns `done: false, timedOut: true` along with the current state and any output drained so far, and the agent calls again to keep waiting. Because draining uses the same cursor as `log --new`, resuming never re-reads output it already saw.

See [Install](../README.md#use-with-ai-agents-mcp) for registering with other MCP clients.

## Other Commands

### `yoink version`

Show version information.

```bash
yoink version
```

### `yoink daemon`

Start the daemon. This is normally auto-started by other commands.

```bash
yoink daemon
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose` | Enable verbose logging |
