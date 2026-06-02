# Killer Examples

These are compact workflows where `yoink` replaces brittle shell glue with named, scriptable PTY sessions.

For longer versions, see [Examples](../EXAMPLES.md).

## CI: Dev Server + E2E Tests

CI often needs a web server running while Playwright or Cypress tests execute. Plain `npm run dev &` makes it awkward to know whether the server is ready, capture the right logs, and clean up reliably.

```bash
yoink run --alias app "npm run dev"

ready=0
for i in $(seq 1 60); do
  if yoink log app --new | grep -q "Local:.*http://localhost:3000"; then
    ready=1
    break
  fi
  sleep 1
done

if [ "$ready" -ne 1 ]; then
  yoink log app
  yoink kill app
  exit 1
fi

yoink run --alias e2e "npx playwright test --reporter=list"
yoink wait e2e --timeout 300 --poll 5
status=$?

if [ "$status" -ne 0 ]; then
  yoink log app --new
fi

yoink kill app
yoink clean
exit "$status"
```

Why it hurts without yoink: readiness checks, server logs, test logs, and cleanup usually become separate ad hoc scripts.

## CI: Parallel Build Matrix With Labeled Logs

Shell background jobs can parallelize work, but debugging failures is painful when logs interleave or disappear.

```bash
yoink run --alias build-api "make build-api"
yoink run --alias build-web "make build-web"
yoink run --alias build-worker "make build-worker"

yoink wait build-api build-web build-worker --timeout 600 --poll 5
```

Why it hurts without yoink: `wait` only tells you that something failed; yoink keeps each command named, stateful, and inspectable.

## CI: Start Slow Setup Early, Join Later

Some CI steps are slow because they download dependencies, pull models, warm caches, build assets, or boot local services. If later steps do not need them immediately, start them with `yoink`, keep doing other work, then wait only when the pipeline actually depends on the result.

```yaml
name: test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: stable

      - name: Install yoink
        run: |
          go install github.com/AxeForging/yoink@latest
          echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"

      - name: Start slow setup
        run: |
          yoink run --alias pg-image "docker pull postgres:16"
          yoink run --alias redis-image "docker pull redis:7"
          yoink run --alias web-deps "cd web && npm ci"

      - name: Do independent checks while setup runs
        run: |
          test -z "$(gofmt -l .)"
          go vet ./...
          go test ./internal/...

      - name: Join slow setup
        run: |
          yoink wait pg-image redis-image web-deps --timeout 900 --poll 10

      - name: Run tests that need the setup
        run: |
          docker compose up -d postgres redis
          (cd web && npm test)
          (cd web && npx playwright test)
```

Why it hurts without yoink: shell `&` can start work in the background, but later steps lose a clean handle for logs, state, exit codes, and timeout behavior. yoink turns those slow setup tasks into named CI resources you can inspect and join later.

## CI: Interactive Infrastructure Approval

Infrastructure tools sometimes need a plan review before confirmation. That is a bad fit for fully headless scripts and a bad fit for manual terminal babysitting.

```bash
yoink run --alias infra "pulumi up --stack prod"
sleep 10

yoink log infra --new
yoink send infra "details"
sleep 5
yoink log infra --new

yoink send infra "yes"
yoink wait infra --timeout 900 --poll 15 \
  --message "Infrastructure apply exceeded 15min"
```

Why it hurts without yoink: `yes | pulumi up` skips review, while a manual run cannot be controlled cleanly by CI.

## CI: Secret Entry For PTY-Aware Prompts

Some tools demand secrets through an interactive prompt and will not behave correctly with a normal stdin pipe.

```bash
yoink run --alias release "vendor-cli deploy --prod"
sleep 2

yoink send-redacted release "$DEPLOY_TOKEN"
yoink wait release --timeout 300
```

Why it hurts without yoink: `echo "$TOKEN" | command` can fail with PTY-aware prompts and can leak secrets through logs or shell history.

## Local Dev: Port-Forward Stack

Multiple `kubectl port-forward` processes are easy to start and easy to lose.

```bash
yoink run --alias pf-api "kubectl port-forward svc/api 8080:80"
yoink run --alias pf-db "kubectl port-forward svc/postgres 5432:5432"
yoink run --alias pf-grafana "kubectl port-forward svc/grafana 3000:3000"

yoink ls
yoink log pf-api --new

yoink kill pf-api
yoink kill pf-db
yoink kill pf-grafana
yoink clean
```

Why it hurts without yoink: when a port-forward drops, a background shell job rarely tells you clearly what died or why.

## AI Agents: Multiple Interactive Workers

AI coding agents are long-running, interactive, and often TUI-based. They need real terminals, but you still want to coordinate them from scripts.

```bash
yoink run --alias agent-api "cd worktree-api && claude"
yoink run --alias agent-tests "cd worktree-tests && claude"

yoink send agent-api "Add pagination to GET /users" --submit
yoink send agent-tests "Wait for the API shape, then write tests" --submit

yoink snapshot agent-api
yoink wait agent-api --timeout 900 --poll 10

yoink send agent-tests "API is done. Use query params page and limit." --submit
yoink snapshot agent-tests
```

Why it hurts without yoink: terminal multiplexers are manual; yoink gives scripts a way to observe, steer, and reattach.

## TUI Monitoring: Render The Screen, Not Escape Codes

TUIs write terminal control sequences, so logs are usually unreadable. `snapshot` gives you the rendered screen.

```bash
yoink run --alias gpu "watch -n 5 nvidia-smi"
yoink snapshot gpu

yoink run --alias train "python train.py"
yoink log train --new
```

Why it hurts without yoink: raw PTY logs are not the same thing as the screen the user sees.

## Stdio Server Harness

Stdio-based tools such as MCP servers need stdin/stdout held open while you send structured messages and inspect responses.

```bash
yoink run --alias mcp "node dist/mcp-server.js --stdio"
sleep 1

yoink send mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
yoink log mcp --new

yoink send mcp '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search","arguments":{"query":"test"}}}'
yoink log mcp --new
```

Why it hurts without yoink: you otherwise end up writing a custom harness just to keep the process alive and talk to it.
