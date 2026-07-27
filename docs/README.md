# yoink Documentation

## Guides

- [Why yoink?](WHY.md): positioning, use cases, and comparisons with common alternatives.
- [Killer examples](KILLER_EXAMPLES.md): compact real-world CI and automation cases where yoink removes painful glue code.
- [Command reference](COMMANDS.md): commands, aliases, flags, and behavior notes.
- [Examples](../EXAMPLES.md): practical workflows for CI, prompts, TUIs, tunnels, AI agents, model servers, and more.

## Architecture

yoink uses a daemon/client architecture:

```
yoink run "cmd"               -->  yoinkd (daemon)  -->  PTY --> child process
yoink ls                      -->  yoinkd           -->  process table
yoink log 1                   -->  yoinkd           -->  ring buffer (last N lines)
yoink snapshot 1              -->  yoinkd           -->  VT100 emulator framebuffer (rendered screen)
yoink send 1 "y"              -->  yoinkd           -->  write to PTY fd (whole string + newline)
yoink send 1 "y" --type       -->  yoinkd           -->  write to PTY fd char by char with delay
yoink send 1 "y" --no-enter   -->  yoinkd           -->  write to PTY fd without trailing newline
yoink send 1 "y" --submit     -->  yoinkd           -->  write to PTY fd with carriage-return Enter
yoink send 1 --key up         -->  yoinkd           -->  write ESC[A to PTY fd (named key sequence)
yoink attach 1                -->  yoinkd           -->  bridge terminal <-> PTY

agent (MCP client) <-stdio->  yoink mcp        -->  yoinkd  (same socket, same process table)
```

### Daemon

The daemon (`yoink daemon`) listens on `~/.yoink/yoink.sock`. It manages:
- Process spawning with PTY allocation
- Output ring buffers (1000 lines per process)
- VT100 emulator per process (for `snapshot` support)
- Process lifecycle monitoring
- Attach/detach multiplexing

### Snapshot vs Log

`yoink log` reads raw lines from the ring buffer. For plain processes this is clean text, but TUI applications (htop, vim, any ncurses app) write cursor-movement and erase sequences that make `log` output unreadable.

`yoink snapshot` feeds all PTY bytes through a `vt10x` VT100 emulator maintained per process. At any point you can call `snapshot` to get the current rendered screen as plain text — exactly what you'd see if you were attached to the terminal. It works on running and finished processes alike.

### MCP Server

`yoink mcp` serves the process manager over the Model Context Protocol on stdio. It is a **client of the daemon**, not a second process manager: each tool call becomes the same `domain.DaemonRequest` the CLI sends, over the same socket.

Two consequences follow. Daemon behavior can never drift between the two frontends, since there is only one implementation. And the CLI and an agent share one process table — an agent's `yoink_ls` lists the dev server you started by hand, and you can `yoink attach` to something an agent spawned.

The server lives in `services/mcp_service.go` (construction, stdio serve) and `services/mcp_tools.go` (schemas and handlers), with `actions/mcp.go` as the CLI entry point — the same split as the daemon itself.

stdout is the protocol stream, so nothing on that path may print to it. The logger already writes to stderr; the integration test's handshake would fail if that ever regressed.

### Protocol

Client-daemon communication uses JSON over Unix socket. Each connection handles one request-response pair, except for `attach` which switches to raw byte forwarding after the handshake.

### Platform Abstraction

PTY operations are abstracted via build tags:
- `platform_unix.go`: Uses `creack/pty` for Linux and macOS
- `platform_windows.go`: Stub (ConPTY support planned)
