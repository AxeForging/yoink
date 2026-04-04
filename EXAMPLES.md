# Examples & Real-World Use Cases

Practical scenarios where yoink shines — from CI pipelines to local development workflows.

---

## Real-World Use Cases

<details>
<summary><strong>CI/CD Pipeline Orchestration</strong></summary>

### Problem
You need to run multiple build steps in parallel, wait for all of them, and fail fast if any step breaks — without losing output.

### Solution
```bash
# Kick off parallel builds
yoink run --alias build-api "make build-api"
yoink run --alias build-web "make build-web"
yoink run --alias build-worker "make build-worker"

# Wait for all three, streaming output as it arrives
yoink wait build-api build-web build-worker --timeout 300 --poll 2

# If any build fails, wait exits with its exit code
# If all succeed, exit code is 0
```

### Why yoink?
Standard `&` + `wait` loses output ordering, doesn't stream logs, and makes it hard to debug which step failed. yoink labels output per-process and propagates exit codes.

</details>

<details>
<summary><strong>Answering Interactive Prompts in Scripts</strong></summary>

### Problem
A CLI tool (installer, scaffolder, migration) requires interactive input but you need to automate it in a script or CI environment.

### Solution
```bash
yoink run --alias setup "npx create-next-app@latest my-app"
sleep 2

# Check what it's asking
yoink log setup --new

# Answer the prompts as they appear
yoink send setup "Yes"           # TypeScript?
sleep 1
yoink send setup "Yes"           # ESLint?
sleep 1
yoink send setup "Yes"           # Tailwind?
sleep 1
yoink send setup "No"            # src/ directory?
sleep 1
yoink send setup "Yes"           # App Router?

# Wait for it to finish
yoink wait setup --timeout 120
```

### Why yoink?
Tools like `expect` are brittle and hard to debug. yoink lets you inspect output between sends, making it easy to adapt to changing prompts.

</details>

<details>
<summary><strong>Dev Server + Test Runner Workflow</strong></summary>

### Problem
You need a dev server running in the background while you iterate on tests, and you want to see server logs when something fails.

### Solution
```bash
# Start the dev server
yoink run --alias server "npm run dev"
sleep 3

# Verify it's up
yoink log server --new | grep "ready on"

# Run tests against it
yoink run --alias tests "npm test -- --watch"

# Check test output as you work
yoink log tests --new

# When a test fails, check what the server logged
yoink log server --new

# Done for the day
yoink kill server
yoink kill tests
yoink clean
```

### Why yoink?
Running servers in terminal tabs loses context when you switch. yoink keeps everything in one terminal session with on-demand log access.

</details>

<details>
<summary><strong>Secure Token/Password Entry in Automation</strong></summary>

### Problem
An automated script needs to enter credentials (API keys, passwords) but you don't want them showing up in logs, CI output, or shell history.

### Solution
```bash
yoink run --alias deploy "kubectl exec -it pod -- /bin/sh"
sleep 2

# Send the password — it's masked as ******** in yoink's logs
yoink send-redacted deploy "$DB_PASSWORD"

# Verify it worked without exposing the secret
yoink log deploy --new
# Output shows: [in] ********
#                [out] Authentication successful
```

### Why yoink?
`echo $SECRET | command` leaks to process lists. Piping stdin doesn't work with PTY-aware prompts. `send-redacted` writes directly to the PTY and masks the value everywhere.

</details>

<details>
<summary><strong>Monitoring TUI Applications</strong></summary>

### Problem
You're running a TUI app (htop, k9s, vim, Claude) in the background and need to check its current state programmatically — but `log` gives you raw escape sequences.

### Solution
```bash
# Start a TUI process
yoink run --alias dash "htop"

# Wrong way — escape garbage:
# yoink log dash

# Right way — rendered screen:
yoink snapshot dash

# Use snapshot in scripts for health checks
if yoink snapshot dash | grep -q "load average: [5-9]"; then
  echo "High load detected!"
fi

# Navigate the TUI with named keys
yoink send dash --key f6         # Sort by column
yoink send dash --key down
yoink send dash --key enter
yoink snapshot dash              # See updated view
```

### Why yoink?
No other background process manager gives you a rendered screen capture of TUI apps. yoink replays VT100 escape sequences through a terminal emulator and returns clean text.

</details>

<details>
<summary><strong>Long-Running Migrations with Timeout Guards</strong></summary>

### Problem
Database migrations or data backfills can hang silently. You need a timeout that kills the process and alerts, plus a way to check progress mid-flight.

### Solution
```bash
yoink run --alias migrate "python manage.py migrate --run-syncdb"

# Check progress periodically
yoink log migrate --new

# Set a hard timeout with a custom error message
yoink wait migrate \
  --timeout 600 \
  --poll 10 \
  --message "Migration exceeded 10min budget — check for locks"

# Exit code 124 = timeout, process exit code = failure, 0 = success
echo "Migration exit code: $?"
```

### Why yoink?
Raw `timeout` command doesn't stream output while waiting. yoink streams progress on each poll cycle so you can watch the migration in real time.

</details>

<details>
<summary><strong>Driving AI Agents and Chat CLIs</strong></summary>

### Problem
You want to script interactions with a conversational CLI (Claude, ChatGPT CLI, database REPLs) — send prompts, read responses, and react to output.

### Solution
```bash
yoink run --alias claude "claude"
sleep 3

# Send a prompt with realistic typing
yoink send claude "Explain this error: ConnectionRefused on port 5432" --type --delay 30

# Wait for response to appear, then read it
sleep 10
yoink snapshot claude

# Send follow-up
yoink send claude "How do I fix it?" --type

# Navigate through output
yoink send claude --key pageup
yoink snapshot claude
```

### Why yoink?
Chat CLIs need a real PTY to function properly. yoink provides one, plus `snapshot` gives you the rendered conversation instead of raw terminal codes.

</details>

<details>
<summary><strong>Multi-Service Integration Testing</strong></summary>

### Problem
Integration tests need multiple services (API, worker, database) running simultaneously. You need to start them, verify they're healthy, run tests, and tear everything down.

### Solution
```bash
# Start all services
yoink run --alias db "docker compose up postgres redis"
yoink run --alias api "cargo run --bin api"
yoink run --alias worker "cargo run --bin worker"

# Wait for readiness (check logs for markers)
sleep 5
yoink log db --new | grep -q "ready to accept connections"
yoink log api --new | grep -q "listening on :8080"
yoink log worker --new | grep -q "connected to queue"

# Run the integration suite
yoink run --alias tests "cargo test --test integration"
yoink wait tests --timeout 300

# If tests fail, grab all service logs for debugging
if [ $? -ne 0 ]; then
  echo "=== API logs ===" && yoink log api
  echo "=== Worker logs ===" && yoink log worker
  echo "=== DB logs ===" && yoink log db
fi

# Tear down
yoink kill worker
yoink kill api
yoink kill db
yoink clean
```

### Why yoink?
Docker Compose handles containers, but what about local binaries? yoink manages any process — containerized or not — with consistent log access and lifecycle control.

</details>

<details>
<summary><strong>Asciinema Recording with Simulated Typing</strong></summary>

### Problem
You're recording a terminal demo with asciinema and want realistic-looking typed input without actually typing it yourself.

### Solution
```bash
# Start recording in a yoink-managed shell
yoink run --alias demo "asciinema rec demo.cast"
sleep 2

# Simulate typing with natural delay
yoink send demo "yoink run --alias server 'python -m http.server'" --type --delay 80
sleep 2

yoink send demo "yoink ls" --type --delay 60
sleep 2

yoink send demo "curl localhost:8000" --type --delay 70
sleep 3

yoink send demo "yoink kill server" --type --delay 60
sleep 1

# End the recording
yoink send demo "exit" --type
```

### Why yoink?
`--type --delay` sends characters one at a time with configurable timing, producing natural-looking recordings without manual typing or complex scripting.

</details>

<details>
<summary><strong>Local LLM Serving & Inference</strong></summary>

### Problem
You want to run a local model server (Ollama, vLLM, llama.cpp) in the background, wait for it to load, and then hit it from scripts or other tools — without blocking your terminal.

### Solution
```bash
# Start Ollama in the background
yoink run --alias ollama "ollama serve"
sleep 3

# Pull a model — this takes a while
yoink run --alias pull "ollama pull qwen3:32b"
yoink wait pull --timeout 600 --poll 10

# Verify model is loaded
yoink log pull --new | grep "success"

# Run inference in a conversational session
yoink run --alias chat "ollama run qwen3:32b"
sleep 2

yoink send chat "Summarize this error log in one sentence: $(cat /tmp/crash.log)"
sleep 15
yoink log chat --new

# Send follow-up
yoink send chat "What's the root cause?"
sleep 10
yoink log chat --new

# Kill the session but keep the server
yoink kill chat
```

### Why yoink?
Model servers take time to load and need to stay alive across workflows. yoink lets you start the server once, confirm it's ready, and interact with it from anywhere — scripts, CI, or other yoink processes.

</details>

<details>
<summary><strong>Kubernetes Port-Forward Juggling</strong></summary>

### Problem
Debugging microservices locally means managing a pile of `kubectl port-forward` processes. They crash silently, and you lose track of which ports map to which service.

### Solution
```bash
# Forward all the services you need
yoink run --alias pf-api "kubectl port-forward svc/api 8080:80"
yoink run --alias pf-redis "kubectl port-forward svc/redis 6379:6379"
yoink run --alias pf-postgres "kubectl port-forward svc/postgres 5432:5432"
yoink run --alias pf-grafana "kubectl port-forward svc/grafana 3000:3000"

# Quick health check — are they all still alive?
yoink ls

# When a forward dies (it always does), check why and restart
yoink log pf-api --new
# "error: lost connection to pod"
yoink kill pf-api
yoink run --alias pf-api "kubectl port-forward svc/api 8080:80"

# Done debugging — tear down all forwards
yoink kill pf-api pf-redis pf-postgres pf-grafana
yoink clean
```

### Why yoink?
Port-forwards in background shells are invisible when they break. yoink gives each one a name, tracks its state, and shows you the error when the connection drops.

</details>

<details>
<summary><strong>Infrastructure Applies with Approval Gates</strong></summary>

### Problem
Running `pulumi up` or `terraform apply` in automation requires reviewing the plan and then confirming — an interactive step that doesn't work in headless scripts.

### Solution
```bash
# Start the apply
yoink run --alias infra "pulumi up --stack prod"
sleep 10

# Review the plan output
yoink log infra --new
# Shows: "Do you want to perform this update? [yes/no/details]"

# Inspect details first
yoink send infra "details"
sleep 5
yoink log infra --new

# Looks good — approve it
yoink send infra "yes"

# Wait for the apply with a generous timeout
yoink wait infra --timeout 900 --poll 15 \
  --message "Pulumi apply exceeded 15min — possible stuck resource"

echo "Apply exit code: $?"
```

### Why yoink?
IaC tools need interactive confirmation for production changes. yoink bridges the gap between "review before applying" and "run it in a script" — you get both.

</details>

<details>
<summary><strong>ML Training Run Monitoring</strong></summary>

### Problem
Training runs take hours or days. You need to launch them, check loss/metrics periodically, and catch hangs or NaN losses early — without babysitting a terminal.

### Solution
```bash
# Start training
yoink run --alias train "python train.py --epochs 100 --lr 3e-4 --wandb"

# Periodically check progress
yoink log train --new
# "Epoch 12/100 | loss: 0.342 | val_acc: 0.891 | lr: 2.7e-4"

# Watch GPU utilization alongside
yoink run --alias gpu "watch -n 5 nvidia-smi"
yoink snapshot gpu
# See real GPU memory/utilization as a rendered table

# Set a timeout guard for the full run
yoink wait train --timeout 86400 --poll 60 \
  --message "Training exceeded 24h budget — possible hang"

# Check final metrics
yoink log train --lines 20
```

### Why yoink?
Training scripts in tmux sessions get lost. `nvidia-smi` in another tab requires switching. yoink keeps both in one session — `log` for training output, `snapshot` for the GPU dashboard.

</details>

<details>
<summary><strong>MCP Server Lifecycle</strong></summary>

### Problem
You're developing or testing MCP (Model Context Protocol) servers and need to start them, verify they initialize, test tool calls, and restart on code changes.

### Solution
```bash
# Start your MCP server
yoink run --alias mcp "node dist/mcp-server.js --stdio"
sleep 2

# Check it initialized properly
yoink log mcp --new
# "MCP server started, tools: [search, fetch, analyze]"

# Test a tool call by sending JSON-RPC over stdin
yoink send mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
sleep 1
yoink log mcp --new

# Send a tool invocation
yoink send mcp '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search","arguments":{"query":"test"}}}'
sleep 2
yoink log mcp --new

# Code changed — restart the server
yoink kill mcp
yoink run --alias mcp "node dist/mcp-server.js --stdio"
```

### Why yoink?
MCP servers using stdio transport need a process to hold their stdin/stdout open. yoink acts as the host process, letting you send JSON-RPC messages and read responses without writing a test harness.

</details>

<details>
<summary><strong>SSH Tunnel Management</strong></summary>

### Problem
You need multiple SSH tunnels open to reach databases, internal APIs, or dashboards behind a bastion — and they randomly drop.

### Solution
```bash
# Tunnel to production database through bastion
yoink run --alias tunnel-db \
  "ssh -N -L 5433:prod-db.internal:5432 bastion.example.com"

# Tunnel to internal monitoring
yoink run --alias tunnel-grafana \
  "ssh -N -L 3001:grafana.internal:3000 bastion.example.com"

# Verify tunnels are alive
yoink ls
# ID  ALIAS           STATE    PID
# 1   tunnel-db       running  48291
# 2   tunnel-grafana  running  48305

# Connect to the tunneled database
psql -h localhost -p 5433 -U readonly prod

# If a tunnel drops, yoink shows the exit state
yoink ls
# ID  ALIAS           STATE       EXIT
# 1   tunnel-db       failed      255

# Check why and restart
yoink log tunnel-db --new
yoink kill tunnel-db
yoink run --alias tunnel-db \
  "ssh -N -L 5433:prod-db.internal:5432 bastion.example.com"
```

### Why yoink?
SSH tunnels in `&` background jobs give no feedback when they die. yoink tracks their state, shows exit codes, and preserves error output so you know exactly what broke.

</details>

<details>
<summary><strong>E2E Tests with Dev Server Lifecycle</strong></summary>

### Problem
Playwright/Cypress E2E tests need a fully booted dev server. CI scripts use hacky `wait-on` or `sleep 30` to guess when the server is ready.

### Solution
```bash
# Start the dev server
yoink run --alias app "npm run dev"

# Poll until the server is actually ready (not just started)
for i in $(seq 1 30); do
  if yoink log app --new | grep -q "Local:.*http://localhost:3000"; then
    echo "Server ready"
    break
  fi
  sleep 1
done

# Run Playwright
yoink run --alias e2e "npx playwright test --reporter=list"
yoink wait e2e --timeout 300 --poll 5

# On failure, grab server-side errors for the test report
if [ $? -ne 0 ]; then
  echo "--- Server logs during test run ---"
  yoink log app --new
fi

yoink kill app
yoink clean
```

### Why yoink?
The server and tests are correlated — when a test fails, you need the server logs from that exact window. yoink's `--new` flag gives you just the logs from during the test run, not the entire server history.

</details>

<details>
<summary><strong>Multi-Agent AI Orchestration</strong></summary>

### Problem
You're coordinating multiple AI coding agents (Claude Code, Aider, Codex) working on different parts of a codebase, and need to monitor their progress and feed them results from each other.

### Solution
```bash
# Start agents on different tasks in separate worktrees
yoink run --alias agent-api \
  "cd worktree-api && claude -p 'Add pagination to the /users endpoint'"

yoink run --alias agent-tests \
  "cd worktree-tests && claude -p 'Write integration tests for the users API'"

yoink run --alias agent-docs \
  "cd worktree-docs && claude -p 'Update API docs for the users endpoint'"

# Monitor progress with snapshots (Claude is a TUI)
yoink snapshot agent-api
yoink snapshot agent-tests

# Wait for the API agent to finish first — tests depend on its output
yoink wait agent-api --timeout 600 --poll 10

# Feed the API changes to the test agent
yoink send agent-tests "The API agent is done. The new endpoint is GET /users?page=N&limit=N. Update tests accordingly."

# Wait for remaining agents
yoink wait agent-tests agent-docs --timeout 600
```

### Why yoink?
AI agents are long-running, interactive, and need to be coordinated. yoink gives each agent a named PTY, lets you monitor via `snapshot`, and pipe context between them with `send`.

</details>

<details>
<summary><strong>Docker Build + Push Matrix</strong></summary>

### Problem
You're building multi-arch container images or pushing to multiple registries, and want to parallelize while tracking which builds succeed.

### Solution
```bash
# Parallel multi-arch builds
yoink run --alias build-amd64 \
  "docker build --platform linux/amd64 -t myapp:amd64 ."
yoink run --alias build-arm64 \
  "docker build --platform linux/arm64 -t myapp:arm64 ."

# Wait for both
yoink wait build-amd64 build-arm64 --timeout 600

# Create and push manifest
yoink run --alias push "docker manifest create myapp:latest \
  myapp:amd64 myapp:arm64 && docker manifest push myapp:latest"
yoink wait push --timeout 120

# Check what happened
yoink log build-amd64 --lines 5
yoink log build-arm64 --lines 5
yoink log push --lines 5

yoink clean
```

### Why yoink?
`docker build &` runs blind — you can't tell which arch failed or see its output. yoink labels each build, streams logs, and propagates the first failure via `wait`.

</details>

---

## Quick Patterns

| Pattern | Command |
|---------|---------|
| Run in background | `yoink run --alias name "command"` |
| Check new output | `yoink log name --new` |
| Send input | `yoink send name "text"` |
| Send secret input | `yoink send-redacted name "$SECRET"` |
| Send key combo | `yoink send name --key ctrl+c` |
| Wait with timeout | `yoink wait name --timeout 60` |
| Wait for multiple | `yoink wait a b c --timeout 120` |
| View TUI screen | `yoink snapshot name` |
| Interactive reattach | `yoink attach name` (Ctrl+] to detach) |
| Kill gracefully | `yoink kill name` |
| Force kill | `yoink kill name --force` |
| Cleanup finished | `yoink clean` |
