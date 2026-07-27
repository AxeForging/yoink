# 001 — MCP server (`yoink mcp`)

Status: implemented

## Problem

Agents drive yoink today by shelling out to the CLI and reading the terminal
output back. That works, but it has three sharp edges:

1. **State has to be parsed out of a table.** `yoink ls` prints an aligned
   text grid. An agent deciding "is the build done?" is doing string surgery on
   a display format that exists for humans.
2. **The useful behaviors are undiscoverable.** `log --new` (incremental
   polling) and `snapshot` (rendered TUI screen, instead of escape-sequence
   soup) are the two things that make yoink worth using from an agent, and both
   only get discovered if someone loads a skill file that mentions them.
3. **`wait` has no equivalent.** Blocking on N processes with a timeout is the
   one thing an agent genuinely cannot express as a fire-and-forget shell call.

## Approach

Serve the existing daemon protocol over the Model Context Protocol on stdio, as
a `yoink mcp` subcommand of the same binary.

The MCP server is a **client of the daemon**, not a second process manager. It
builds `domain.DaemonRequest` values and sends them through the same
`services.ClientService` the CLI uses. Two consequences, both wanted:

- Zero divergence. Any daemon behavior fix lands in both frontends at once.
- **One shared daemon.** An agent's `yoink_ls` sees the process a human started
  in their own shell, and vice versa. The CLI and the agent are looking at the
  same board.

This mirrors the existing `daemon` command shape: server logic in `services/`,
a thin `actions/` entry point that wires and starts it.

### Distribution

No new artifact. The server compiles into the existing binary, so `install.sh`,
the GitHub Action, and `go install` all deliver it unchanged. Users register it
with one command:

```bash
claude mcp add --scope user yoink -- yoink mcp
```

## Tool surface

Eight tools. The CLI has more commands than this; the extra ones are
deliberately not exposed.

| Tool | Maps to | Notes |
|------|---------|-------|
| `yoink_run` | `ReqRun` | command, alias, cwd, log_file |
| `yoink_ls` | `ReqList` | structured `ProcessInfo[]`, always full detail |
| `yoink_log` | `ReqLog` | `new: true` for incremental polling |
| `yoink_send` | `ReqSend` | text or `key`, plus redacted/submit/no_enter/type |
| `yoink_snapshot` | `ReqSnapshot` | rendered VT100 screen for TUIs |
| `yoink_wait` | `ReqList` + `ReqLog` polling | multi-process, bounded (see below) |
| `yoink_kill` | `ReqKill` | force → SIGKILL |
| `yoink_clean` | `ReqClean` | all/force |

**Not exposed, and why:**

- `attach` — needs a real controlling TTY to be meaningful. Over stdio it is
  either a no-op or a hang.
- `daemon` — an implementation detail; the client auto-starts it.
- `version` — reported in the MCP initialize handshake already.
- `send-redacted` — a `redacted` flag on `yoink_send`, not a separate tool. Same
  for `--type`/`--delay`. Fewer schemas, same capability.

### `yoink_wait` and client timeouts

MCP clients cap how long a single tool call may run, typically around 60s. A
naive `yoink_wait` with `--timeout 300` would be killed by the client and
surface as a transport error rather than a useful result.

So `yoink_wait` is **bounded and resumable**:

- `timeout_seconds` defaults to 30 and is clamped to 55.
- Exceeding it is **not an error**. The tool returns `success` with
  `timed_out: true`, the current state of every process, and any output drained
  so far.
- The agent re-calls to keep waiting. Because output draining uses the same
  `--new` cursor as the CLI, re-calling never re-reads lines it already saw.

A process that *fails* is likewise a normal result (`state: "failed"` with its
`exit_code`), not a tool error. Tool errors are reserved for "the request was
malformed or the daemon is unreachable" — things the agent should fix rather
than interpret.

### stdout discipline

stdio is the MCP transport, so anything written to stdout corrupts the protocol
stream. `helpers.SetupLogger` already writes to stderr, and the MCP path prints
nothing itself. The integration test's handshake-and-call would break if this
ever regressed, which is the point of running it against the real binary.

## Test plan

Everything runs real — real daemon, real PTYs, real processes. No mocks.

- **`services/mcp_service_test.go`** — the SDK server connected to an
  in-memory transport, pointed at a real daemon via the existing
  `startTestDaemon` helper.
  - tool list is complete and stable
  - run → ls → log round-trip reports real output and exit state
  - `log` with `new: true` returns fresh lines once and not twice
  - `send` reaches a process that is actually reading stdin
  - `wait` returns `timed_out: true` (not an error) for a process still running
  - `wait` reports `exit_code` for a process that failed
  - `snapshot` renders a screen rather than escape bytes
  - `kill` then `clean --all` empties the list
  - negative: unknown alias, missing required args, mutually exclusive
    `input` + `key`
- **`integration/mcp_test.go`** — builds the binary and speaks MCP over stdio
  to `yoink mcp`, proving the handshake, the tool list, and a real run/log
  round-trip through the actual shipped entry point.

## Checklist

- [x] `services/mcp_service.go` — server construction, stdio serve
- [x] `services/mcp_tools.go` — tool schemas and handlers
- [x] `services/process_ref.go` — `ParseProcessRef` shared with `actions`
- [x] `actions/mcp.go` — CLI entry point
- [x] `main.go` — register the `mcp` command
- [x] unit tests over in-memory transport
- [x] integration test over stdio against the built binary
- [x] README install/registration docs, `docs/COMMANDS.md` entry
