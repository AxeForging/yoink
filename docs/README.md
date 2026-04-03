# yoink Documentation

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
yoink attach 1                -->  yoinkd           -->  bridge terminal <-> PTY
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

### Protocol

Client-daemon communication uses JSON over Unix socket. Each connection handles one request-response pair, except for `attach` which switches to raw byte forwarding after the handshake.

### Platform Abstraction

PTY operations are abstracted via build tags:
- `platform_unix.go`: Uses `creack/pty` for Linux and macOS
- `platform_windows.go`: Stub (ConPTY support planned)
