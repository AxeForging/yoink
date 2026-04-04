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
