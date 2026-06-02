# Why yoink?

`yoink` is for commands that are too interactive for plain background jobs and too automation-heavy for a manual terminal multiplexer.

It gives every managed command a real PTY, a stable name, buffered output, lifecycle state, input control, timeout-aware waiting, rendered TUI snapshots, and an escape hatch back into full interactive control.

## The Problem

Developers constantly run commands that do not fit neatly into one terminal session:

- deploys that ask for confirmation halfway through
- dev servers that need to stay alive while tests run
- migrations, model pulls, builds, training jobs, and backfills that take a long time
- `kubectl port-forward`, SSH tunnels, and local daemons that fail silently
- TUI programs where raw logs are useless
- AI coding agents and chat CLIs that need a real terminal

The common workaround is a pile of terminal tabs, shell `&`, `nohup`, `tmux`, `screen`, `expect`, and ad hoc scripts. Those tools are useful, but the seams show when you want to script the workflow: start something, check only new output, send a key, wait with a timeout, inspect a rendered screen, then reattach if needed.

## What yoink Adds

`yoink` treats interactive processes as named, scriptable terminal sessions:

```bash
yoink run --alias app "npm run dev"
yoink log app --new
yoink send app --key ctrl+c
yoink wait app --timeout 300
yoink attach app
```

The important part is that the process still thinks it is attached to a terminal. That means prompts, TUIs, colored output, terminal control sequences, and REPLs behave much closer to how they behave in a real shell.

## Compared To Alternatives

| Tool | Great for | Where yoink fits |
|------|-----------|------------------|
| Shell `&` / `jobs` | Quick one-off backgrounding | Adds names, logs, process state, input, timeouts, and reattach |
| `nohup` | Keeping non-interactive commands alive | Keeps a real PTY and supports later interaction |
| `tmux` / `screen` | Manual long-lived terminal sessions | Provides script-friendly commands around PTY sessions |
| `expect` | Automating known prompt sequences | Lets scripts inspect, react, send keys, and debug incrementally |
| `timeout` | Killing commands after a deadline | Streams output while waiting and preserves process logs |
| Process supervisors | Running services reliably | Targets developer workflows, prompts, TUIs, and temporary commands |
| Docker Compose | Container service groups | Also manages local binaries, CLIs, tunnels, and test runners |

`yoink` does not replace these tools. It gives you a small CLI vocabulary for the annoying middle ground: interactive programs that need to be backgrounded, observed, poked, waited on, and occasionally reattached.

## Best Fits

- CI scripts that need to run several commands and keep useful logs.
- E2E test setup where a dev server must be started, checked, and torn down.
- Deploys and infrastructure applies that require approval gates.
- Long-running builds, migrations, model pulls, or ML training jobs.
- Port-forward and tunnel management during debugging.
- TUI monitoring with `snapshot`.
- AI agent orchestration where each agent runs in its own PTY.
- Demo recording with typed input via `send --type --delay`.

## Mental Model

Think of `yoink` as:

> named, scriptable PTY sessions with logs, input, snapshots, waits, and reattach.

That is the core value. It is intentionally smaller than a terminal multiplexer and more interactive than a process supervisor.
