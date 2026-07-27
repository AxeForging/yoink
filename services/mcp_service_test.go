//go:build !windows

package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AxeForging/yoink/domain"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// connectMCP wires the real MCP server to a real daemon over an in-memory
// transport. Nothing is mocked: the daemon spawns actual processes on real PTYs.
func connectMCP(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	_, sockPath := startTestDaemon(t)
	return connectMCPTo(t, sockPath)
}

// connectMCPTo connects to an MCP server backed by an already-running daemon,
// for tests that need to touch that daemon directly as well.
func connectMCPTo(t *testing.T, sockPath string) *mcpsdk.ClientSession {
	t.Helper()
	srv := NewMCPServer(NewClientService(sockPath), "test").sdkServer()
	serverT, clientT := mcpsdk.NewInMemoryTransports()

	ctx := context.Background()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "yoink-test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() {
		cs.Close() //nolint:errcheck
		ss.Wait()  //nolint:errcheck
	})
	return cs
}

// call invokes a tool and fails the test if it returns a tool error.
func call(t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res := callRaw(t, cs, name, args)
	if res.IsError {
		t.Fatalf("%s returned error: %s", name, errorText(res))
	}
	if out == nil {
		return
	}
	decodeResult(t, res, out)
}

// callRaw invokes a tool and returns the result without asserting success, so
// negative cases can inspect the failure.
func callRaw(t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

func decodeResult(t *testing.T, res *mcpsdk.CallToolResult, v any) {
	t.Helper()
	if res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structured content: %v", err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatalf("decode structured content: %v", err)
		}
		return
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), v); err != nil {
				t.Fatalf("decode text content %q: %v", tc.Text, err)
			}
			return
		}
	}
	t.Fatal("result had no decodable content")
}

func errorText(res *mcpsdk.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// runProcess starts a command through the MCP server and returns its id.
func runProcess(t *testing.T, cs *mcpsdk.ClientSession, alias, command string) int {
	t.Helper()
	var out struct {
		ID  int    `json:"id"`
		Cwd string `json:"cwd"`
	}
	call(t, cs, "yoink_run", map[string]any{"command": command, "alias": alias}, &out)
	if out.ID < 1 {
		t.Fatalf("expected a positive process id, got %d", out.ID)
	}
	return out.ID
}

// awaitFinished polls yoink_ls until the process leaves the running state.
func awaitFinished(t *testing.T, cs *mcpsdk.ClientSession, alias string) domain.ProcessInfo {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var out struct {
			Processes []domain.ProcessInfo `json:"processes"`
		}
		call(t, cs, "yoink_ls", map[string]any{}, &out)
		for _, p := range out.Processes {
			if p.Alias == alias && p.State != domain.StateRunning {
				return p
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("process %q did not finish within 10s", alias)
	return domain.ProcessInfo{}
}

func TestMCP_ExposesTheDocumentedToolSurface(t *testing.T) {
	cs := connectMCP(t)

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %s has no description; descriptions are the agent-facing docs", tool.Name)
		}
	}

	want := []string{
		"yoink_run", "yoink_ls", "yoink_log", "yoink_send",
		"yoink_snapshot", "yoink_wait", "yoink_kill", "yoink_clean",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing tool %s", name)
		}
	}
	if len(res.Tools) != len(want) {
		t.Errorf("expected exactly %d tools, got %d (%v)", len(want), len(res.Tools), got)
	}

	// attach needs a controlling TTY and daemon is an implementation detail;
	// neither belongs on the agent surface.
	for _, name := range []string{"yoink_attach", "yoink_daemon", "yoink_send_redacted"} {
		if got[name] {
			t.Errorf("tool %s should not be exposed over MCP", name)
		}
	}
}

func TestMCP_RunReportsRealOutputAndExitState(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "greet", "echo mcp-hello")
	proc := awaitFinished(t, cs, "greet")

	if proc.State != domain.StateDone {
		t.Errorf("expected state done, got %s", proc.State)
	}
	if proc.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", proc.ExitCode)
	}

	var logOut struct {
		Output string `json:"output"`
	}
	call(t, cs, "yoink_log", map[string]any{"process": "greet"}, &logOut)
	if !strings.Contains(logOut.Output, "mcp-hello") {
		t.Errorf("log did not contain the process output; got %q", logOut.Output)
	}
}

func TestMCP_RunReportsFailureStateAndExitCode(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "boom", "sh -c 'exit 7'")
	proc := awaitFinished(t, cs, "boom")

	if proc.State != domain.StateFailed {
		t.Errorf("expected state failed, got %s", proc.State)
	}
	if proc.ExitCode != 7 {
		t.Errorf("expected exit code 7, got %d", proc.ExitCode)
	}
}

func TestMCP_RunResolvesRelativeCwdAgainstServerDirectory(t *testing.T) {
	cs := connectMCP(t)

	// A relative cwd must land somewhere absolute, so the daemon (which may have
	// been started from anywhere) does not decide it.
	var out struct {
		ID  int    `json:"id"`
		Cwd string `json:"cwd"`
	}
	call(t, cs, "yoink_run", map[string]any{
		"command": "pwd", "alias": "rel", "cwd": ".",
	}, &out)

	if !filepath.IsAbs(out.Cwd) {
		t.Errorf("expected an absolute cwd in the result, got %q", out.Cwd)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if out.Cwd != wd {
		t.Errorf("expected cwd %q, got %q", wd, out.Cwd)
	}
}

func TestMCP_RunHonorsAbsoluteCwd(t *testing.T) {
	cs := connectMCP(t)
	dir := t.TempDir()

	call(t, cs, "yoink_run", map[string]any{
		"command": "pwd", "alias": "abs", "cwd": dir,
	}, nil)
	awaitFinished(t, cs, "abs")

	var logOut struct {
		Output string `json:"output"`
	}
	call(t, cs, "yoink_log", map[string]any{"process": "abs"}, &logOut)

	// macOS resolves TempDir through /private, so compare the resolved paths.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}
	if !strings.Contains(logOut.Output, want) {
		t.Errorf("expected process to run in %q; output was %q", want, logOut.Output)
	}
}

func TestMCP_LogNewReturnsEachLineExactlyOnce(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "poll", "echo first-line")
	awaitFinished(t, cs, "poll")

	var first, second struct {
		Output string `json:"output"`
	}
	call(t, cs, "yoink_log", map[string]any{"process": "poll", "new": true}, &first)
	call(t, cs, "yoink_log", map[string]any{"process": "poll", "new": true}, &second)

	if !strings.Contains(first.Output, "first-line") {
		t.Errorf("first new-read should return the output, got %q", first.Output)
	}
	if strings.Contains(second.Output, "first-line") {
		t.Errorf("second new-read re-returned already-seen output: %q", second.Output)
	}
}

func TestMCP_LogRespectsLineLimit(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "many", "sh -c 'for i in 1 2 3 4 5 6 7 8; do echo line-$i; done'")
	awaitFinished(t, cs, "many")

	var out struct {
		Output string `json:"output"`
	}
	call(t, cs, "yoink_log", map[string]any{"process": "many", "lines": 2}, &out)

	if strings.Contains(out.Output, "line-1") {
		t.Errorf("lines:2 should return only the tail, but included the first line: %q", out.Output)
	}
	if !strings.Contains(out.Output, "line-8") {
		t.Errorf("lines:2 should include the last line, got %q", out.Output)
	}
}

func TestMCP_SendDeliversInputToRunningProcess(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "prompt", "sh -c 'read answer; echo received:$answer'")

	// Give the process a moment to reach the read.
	deadline := time.Now().Add(5 * time.Second)
	var sendErr *mcpsdk.CallToolResult
	for time.Now().Before(deadline) {
		sendErr = callRaw(t, cs, "yoink_send", map[string]any{"process": "prompt", "input": "yes-please"})
		if !sendErr.IsError {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if sendErr.IsError {
		t.Fatalf("send never succeeded: %s", errorText(sendErr))
	}

	awaitFinished(t, cs, "prompt")

	var logOut struct {
		Output string `json:"output"`
	}
	call(t, cs, "yoink_log", map[string]any{"process": "prompt"}, &logOut)
	if !strings.Contains(logOut.Output, "received:yes-please") {
		t.Errorf("process did not receive the sent input; log was %q", logOut.Output)
	}
}

func TestMCP_SendKeyDeliversControlSequence(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "sleeper", "sh -c 'sleep 30'")
	time.Sleep(300 * time.Millisecond)

	call(t, cs, "yoink_send", map[string]any{"process": "sleeper", "key": "ctrl+c"}, nil)

	proc := awaitFinished(t, cs, "sleeper")
	if proc.State == domain.StateRunning {
		t.Errorf("ctrl+c did not stop the process")
	}
}

func TestMCP_SnapshotRendersScreenText(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "screen", "echo rendered-screen")
	awaitFinished(t, cs, "screen")

	var out struct {
		Screen string `json:"screen"`
	}
	call(t, cs, "yoink_snapshot", map[string]any{"process": "screen"}, &out)

	if !strings.Contains(out.Screen, "rendered-screen") {
		t.Errorf("snapshot did not render the screen text; got %q", out.Screen)
	}
	if strings.Contains(out.Screen, "\x1b[") {
		t.Errorf("snapshot leaked raw escape sequences: %q", out.Screen)
	}
}

func TestMCP_WaitReturnsResumableTimeoutWhileStillRunning(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "slow", "sh -c 'echo starting; sleep 30'")

	var out mcpWaitOutput
	res := callRaw(t, cs, "yoink_wait", map[string]any{
		"processes": []any{"slow"}, "timeoutSeconds": 1, "pollSeconds": 1,
	})
	// A process that is merely still running is not a tool failure — the agent
	// is expected to call again, not to handle an error.
	if res.IsError {
		t.Fatalf("wait on a running process should not be a tool error: %s", errorText(res))
	}
	decodeResult(t, res, &out)

	if out.Done {
		t.Errorf("expected done=false for a still-running process")
	}
	if !out.TimedOut {
		t.Errorf("expected timedOut=true")
	}
	if out.Failed {
		t.Errorf("expected failed=false for a still-running process")
	}
	if len(out.Processes) != 1 || out.Processes[0].State != string(domain.StateRunning) {
		t.Fatalf("expected one running process, got %+v", out.Processes)
	}
	if out.Processes[0].ID < 1 {
		t.Errorf("timed-out result should still resolve the process id, got %d", out.Processes[0].ID)
	}
	if !strings.Contains(out.Processes[0].Output, "starting") {
		t.Errorf("expected output drained before the timeout, got %q", out.Processes[0].Output)
	}
}

func TestMCP_WaitReportsExitCodeForFailedProcess(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "failing", "sh -c 'echo before-failure; exit 3'")

	var out mcpWaitOutput
	res := callRaw(t, cs, "yoink_wait", map[string]any{
		"processes": []any{"failing"}, "timeoutSeconds": 10, "pollSeconds": 1,
	})
	// A failing process is a result to read, not a transport-level error.
	if res.IsError {
		t.Fatalf("wait should report failure in the result, not as a tool error: %s", errorText(res))
	}
	decodeResult(t, res, &out)

	if !out.Done || out.TimedOut {
		t.Errorf("expected done=true timedOut=false, got done=%v timedOut=%v", out.Done, out.TimedOut)
	}
	if !out.Failed {
		t.Errorf("expected failed=true")
	}
	if len(out.Processes) != 1 {
		t.Fatalf("expected one process, got %d", len(out.Processes))
	}
	p := out.Processes[0]
	if p.State != string(domain.StateFailed) {
		t.Errorf("expected state failed, got %s", p.State)
	}
	if p.ExitCode == nil || *p.ExitCode != 3 {
		t.Errorf("expected exit code 3, got %v", p.ExitCode)
	}
	if !strings.Contains(p.Output, "before-failure") {
		t.Errorf("expected the process output in the wait result, got %q", p.Output)
	}
}

func TestMCP_WaitHandlesMultipleProcesses(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "one", "echo done-one")
	runProcess(t, cs, "two", "sh -c 'sleep 0.2; echo done-two'")

	var out mcpWaitOutput
	call(t, cs, "yoink_wait", map[string]any{
		"processes": []any{"one", "two"}, "timeoutSeconds": 10, "pollSeconds": 1,
	}, &out)

	if !out.Done || out.Failed {
		t.Errorf("expected both to finish successfully, got done=%v failed=%v", out.Done, out.Failed)
	}
	if len(out.Processes) != 2 {
		t.Fatalf("expected two processes, got %d", len(out.Processes))
	}
	for _, p := range out.Processes {
		if p.State != string(domain.StateDone) {
			t.Errorf("process %s: expected done, got %s", p.Process, p.State)
		}
	}
}

func TestMCP_WaitClampsTimeoutToClientSafeMaximum(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "quick", "echo quick")

	var out mcpWaitOutput
	call(t, cs, "yoink_wait", map[string]any{
		"processes": []any{"quick"}, "timeoutSeconds": 600, "pollSeconds": 1,
	}, &out)

	// Reported back so the agent can see the bound it actually got.
	if out.TimeoutSeconds != maxWaitSeconds {
		t.Errorf("expected timeout clamped to %d, got %d", maxWaitSeconds, out.TimeoutSeconds)
	}
}

func TestMCP_WaitDefaultsTimeoutWhenUnset(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "defaulted", "echo defaulted")

	var out mcpWaitOutput
	call(t, cs, "yoink_wait", map[string]any{"processes": []any{"defaulted"}}, &out)

	if out.TimeoutSeconds != defaultWaitSeconds {
		t.Errorf("expected default timeout %d, got %d", defaultWaitSeconds, out.TimeoutSeconds)
	}
}

func TestMCP_KillStopsProcessAndCleanAllEmptiesTheList(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "long", "sh -c 'sleep 30'")
	runProcess(t, cs, "short", "echo short")
	awaitFinished(t, cs, "short")

	var killOut struct {
		ID     int    `json:"id"`
		Signal string `json:"signal"`
	}
	call(t, cs, "yoink_kill", map[string]any{"process": "long"}, &killOut)
	if killOut.Signal != "SIGTERM" {
		t.Errorf("expected SIGTERM, got %s", killOut.Signal)
	}
	if killOut.ID < 1 {
		t.Errorf("kill by alias should report the resolved numeric id, got %d", killOut.ID)
	}
	awaitFinished(t, cs, "long")

	var cleanOut struct {
		Cleaned int `json:"cleaned"`
	}
	call(t, cs, "yoink_clean", map[string]any{"all": true}, &cleanOut)
	if cleanOut.Cleaned != 2 {
		t.Errorf("expected 2 processes cleaned, got %d", cleanOut.Cleaned)
	}

	var listOut struct {
		Processes []domain.ProcessInfo `json:"processes"`
	}
	call(t, cs, "yoink_ls", map[string]any{}, &listOut)
	if len(listOut.Processes) != 0 {
		t.Errorf("expected an empty list after clean --all, got %d", len(listOut.Processes))
	}
}

func TestMCP_KillForceReportsSigkill(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "stubborn", "sh -c 'sleep 30'")
	time.Sleep(200 * time.Millisecond)

	var out struct {
		Signal string `json:"signal"`
	}
	call(t, cs, "yoink_kill", map[string]any{"process": "stubborn", "force": true}, &out)
	if out.Signal != "SIGKILL" {
		t.Errorf("expected SIGKILL, got %s", out.Signal)
	}
	awaitFinished(t, cs, "stubborn")
}

func TestMCP_CleanLeavesRunningProcessesAlone(t *testing.T) {
	cs := connectMCP(t)

	runProcess(t, cs, "keeper", "sh -c 'sleep 30'")
	runProcess(t, cs, "finished", "echo bye")
	awaitFinished(t, cs, "finished")

	var cleanOut struct {
		Cleaned int `json:"cleaned"`
	}
	call(t, cs, "yoink_clean", map[string]any{}, &cleanOut)
	if cleanOut.Cleaned != 1 {
		t.Errorf("expected only the finished process cleaned, got %d", cleanOut.Cleaned)
	}

	var listOut struct {
		Processes []domain.ProcessInfo `json:"processes"`
	}
	call(t, cs, "yoink_ls", map[string]any{}, &listOut)
	if len(listOut.Processes) != 1 || listOut.Processes[0].Alias != "keeper" {
		t.Errorf("expected only the running process to remain, got %+v", listOut.Processes)
	}
}

func TestMCP_ListSeesProcessesStartedOutsideMCP(t *testing.T) {
	// The MCP server is a client of the shared daemon, so it must observe
	// processes a human started from their own terminal.
	_, sockPath := startTestDaemon(t)

	resp := sendRequest(t, sockPath, domain.DaemonRequest{
		Type: domain.ReqRun, Command: "echo from-the-cli", Alias: "cli-side",
	})
	if !resp.Success {
		t.Fatalf("cli-side run failed: %s", resp.Error)
	}

	cs := connectMCPTo(t, sockPath)

	var out struct {
		Processes []domain.ProcessInfo `json:"processes"`
	}
	call(t, cs, "yoink_ls", map[string]any{}, &out)

	var found bool
	for _, p := range out.Processes {
		if p.Alias == "cli-side" {
			found = true
		}
	}
	if !found {
		t.Errorf("MCP did not see the process started via the daemon socket; got %+v", out.Processes)
	}
}

func TestMCP_RejectsInvalidToolArguments(t *testing.T) {
	cs := connectMCP(t)
	runProcess(t, cs, "target", "sh -c 'sleep 30'")

	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{
			name: "unknown alias",
			tool: "yoink_log",
			args: map[string]any{"process": "no-such-process"},
			want: "no process found",
		},
		{
			name: "empty process ref",
			tool: "yoink_log",
			args: map[string]any{"process": ""},
			want: "process is required",
		},
		{
			name: "missing command",
			tool: "yoink_run",
			args: map[string]any{"command": ""},
			want: "command is required",
		},
		{
			name: "input and key together",
			tool: "yoink_send",
			args: map[string]any{"process": "target", "input": "hi", "key": "enter"},
			want: "mutually exclusive",
		},
		{
			name: "neither input nor key",
			tool: "yoink_send",
			args: map[string]any{"process": "target"},
			want: "one of input or key is required",
		},
		{
			name: "noEnter and submit together",
			tool: "yoink_send",
			args: map[string]any{"process": "target", "input": "hi", "noEnter": true, "submit": true},
			want: "mutually exclusive",
		},
		{
			name: "key and submit together",
			tool: "yoink_send",
			args: map[string]any{"process": "target", "key": "enter", "submit": true},
			want: "mutually exclusive",
		},
		{
			name: "unknown key name",
			tool: "yoink_send",
			args: map[string]any{"process": "target", "key": "hyperspace"},
			want: "unknown key",
		},
		{
			name: "wait without processes",
			tool: "yoink_wait",
			args: map[string]any{"processes": []any{}},
			want: "processes is required",
		},
		{
			name: "wait on unknown process",
			tool: "yoink_wait",
			args: map[string]any{"processes": []any{"ghost"}, "timeoutSeconds": 1},
			want: "no process found",
		},
		{
			name: "snapshot of unknown process",
			tool: "yoink_snapshot",
			args: map[string]any{"process": "ghost"},
			want: "no process found",
		},
		{
			name: "kill unknown process",
			tool: "yoink_kill",
			args: map[string]any{"process": "ghost"},
			want: "no process found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := callRaw(t, cs, tc.tool, tc.args)
			if !res.IsError {
				t.Fatalf("expected a tool error, got success")
			}
			if got := errorText(res); !strings.Contains(got, tc.want) {
				t.Errorf("expected error containing %q, got %q", tc.want, got)
			}
		})
	}
}

func TestMCP_ProcessRefAcceptsNumericIDAndAlias(t *testing.T) {
	cs := connectMCP(t)

	id := runProcess(t, cs, "both-ways", "echo addressable")
	awaitFinished(t, cs, "both-ways")

	var byAlias, byID struct {
		Output string `json:"output"`
	}
	call(t, cs, "yoink_log", map[string]any{"process": "both-ways"}, &byAlias)
	call(t, cs, "yoink_log", map[string]any{"process": strconv.Itoa(id)}, &byID)

	if !strings.Contains(byAlias.Output, "addressable") {
		t.Errorf("alias lookup failed: %q", byAlias.Output)
	}
	if byID.Output != byAlias.Output {
		t.Errorf("id and alias lookups disagreed:\n id: %q\n alias: %q", byID.Output, byAlias.Output)
	}
}

func TestParseProcessRef(t *testing.T) {
	tests := []struct {
		ref       string
		wantID    int
		wantAlias string
	}{
		{"7", 7, ""},
		{"api", 0, "api"},
		{"", 0, ""},
		{"0", 0, ""},
		{"12abc", 0, "12abc"},
		{"-1", -1, ""},
	}
	for _, tc := range tests {
		id, alias := ParseProcessRef(tc.ref)
		if id != tc.wantID || alias != tc.wantAlias {
			t.Errorf("ParseProcessRef(%q) = (%d, %q), want (%d, %q)", tc.ref, id, alias, tc.wantID, tc.wantAlias)
		}
	}
}
