# yoink Documentation

## Architecture

yoink uses a daemon/client architecture:

```
yoink run "cmd"  -->  yoinkd (daemon)  -->  PTY --> child process
yoink ls         -->  yoinkd           -->  process table
yoink log 1      -->  yoinkd           -->  ring buffer (last N lines)
yoink send 1 "y" -->  yoinkd           -->  write to PTY fd
yoink attach 1   -->  yoinkd           -->  bridge terminal <-> PTY
```

### Daemon

The daemon (`yoink daemon`) listens on `~/.yoink/yoink.sock`. It manages:
- Process spawning with PTY allocation
- Output ring buffers (1000 lines per process)
- Process lifecycle monitoring
- Attach/detach multiplexing

### Protocol

Client-daemon communication uses JSON over Unix socket. Each connection handles one request-response pair, except for `attach` which switches to raw byte forwarding after the handshake.

### Platform Abstraction

PTY operations are abstracted via build tags:
- `platform_unix.go`: Uses `creack/pty` for Linux and macOS
- `platform_windows.go`: Stub (ConPTY support planned)
