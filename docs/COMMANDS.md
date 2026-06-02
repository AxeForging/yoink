# Command Reference

This page covers the command surface. For longer workflows, see [Examples](../EXAMPLES.md).

## Process Lifecycle

### `yoink run`

Spawn a command under a managed PTY.

```bash
yoink run "make deploy-staging"
yoink run --alias deploy "make deploy-staging"
```

Aliases: `r`

Flags:

| Flag | Description |
|------|-------------|
| `--alias <name>` | Assign a stable name for later commands |

### `yoink ls`

List managed processes with state, PID, aliases, and exit information.

```bash
yoink ls
```

Aliases: `list`, `ps`

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
```

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
