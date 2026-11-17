//go:build windows

package integration

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// waitFor polls `yoink <view> <ref>` until it contains want. Use "snapshot"
// for prompts: the log is line-based, so a prompt without a newline never
// shows up there until the line completes.
func (e *testEnv) waitFor(view, ref, want string) {
	e.t.Helper()
	var out string
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		out, _ = e.run(view, ref)
		if strings.Contains(out, want) {
			return
		}
	}
	e.t.Fatalf("timed out waiting for %q in %s of %s, got:\n%s", want, view, ref, out)
}

func TestIntegration_Windows_RunCapturesOutputThroughConPTY(t *testing.T) {
	env := newTestEnv(t)

	if out, err := env.run("run", "--alias", "hello", "echo hello-windows"); err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	if out, err := env.run("wait", "hello", "--timeout", "20", "--poll", "1"); err != nil {
		t.Fatalf("wait failed: %v\n%s", err, out)
	}
	env.waitFor("log", "hello", "hello-windows")

	// The VT emulator must render ConPTY output into a readable screen.
	snap, err := env.run("snapshot", "hello")
	if err != nil {
		t.Fatalf("snapshot failed: %v\n%s", err, snap)
	}
	if !strings.Contains(snap, "hello-windows") {
		t.Errorf("expected snapshot to show the output, got:\n%s", snap)
	}
}

func TestIntegration_Windows_WaitPropagatesExitCode(t *testing.T) {
	env := newTestEnv(t)

	if out, err := env.run("run", "--alias", "boom", "exit 7"); err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	out, err := env.run("wait", "boom", "--timeout", "20", "--poll", "1")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("expected wait to exit 7, got err=%v\n%s", err, out)
	}
}

// A plain `send` must complete a line for a cooked-mode reader. ConPTY only
// treats CR as Enter, so this fails if send appends a bare LF.
func TestIntegration_Windows_SendAnswersAPrompt(t *testing.T) {
	env := newTestEnv(t)

	// `call` re-expands %NAME% after `set /p` has assigned it.
	if out, err := env.run("run", "--alias", "prompt", "set /p NAME=name? & call echo got-%NAME%"); err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	env.waitFor("snapshot", "prompt", "name?")

	if out, err := env.run("send", "prompt", "yoink"); err != nil {
		t.Fatalf("send failed: %v\n%s", err, out)
	}
	env.waitFor("log", "prompt", "got-yoink")
}

// cmd.exe does not exec its child, so killing only the shell would orphan the
// real workload. kill must take down the whole tree.
func TestIntegration_Windows_KillStopsTheWholeProcessTree(t *testing.T) {
	env := newTestEnv(t)
	const marker = "127.0.0.42" // unique so we only ever match our own ping

	if out, err := env.run("run", "--alias", "pinger", "ping -n 300 "+marker); err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	env.waitFor("log", "pinger", marker)

	if out, err := env.run("kill", "pinger"); err != nil {
		t.Fatalf("kill failed: %v\n%s", err, out)
	}

	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if !pingAlive(t, marker) {
			if ls, _ := env.run("ls"); strings.Contains(ls, "running") {
				t.Fatalf("child is gone but yoink still reports running:\n%s", ls)
			}
			return
		}
	}
	t.Fatal("ping.exe survived yoink kill: the shell was killed but not its child")
}

func pingAlive(t *testing.T, marker string) bool {
	t.Helper()
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-CimInstance Win32_Process -Filter \"Name='PING.EXE'\" | Where-Object CommandLine -like '*"+marker+"*' | Measure-Object | Select-Object -ExpandProperty Count").Output()
	if err != nil {
		t.Fatalf("query processes: %v", err)
	}
	return strings.TrimSpace(string(out)) != "0"
}

func TestIntegration_Windows_MCPRunWaitAndLog(t *testing.T) {
	cs := startMCP(t)

	callTool(t, cs, "yoink_run", map[string]any{"command": "echo mcp-windows & exit 3", "alias": "mcp"}, nil)

	var wait struct {
		Done      bool `json:"done"`
		Failed    bool `json:"failed"`
		Processes []struct {
			ExitCode *int   `json:"exitCode"`
			Output   string `json:"output"`
		} `json:"processes"`
	}
	callTool(t, cs, "yoink_wait", map[string]any{
		"processes": []any{"mcp"}, "timeoutSeconds": 20, "pollSeconds": 1,
	}, &wait)

	if !wait.Done || !wait.Failed || len(wait.Processes) != 1 {
		t.Fatalf("expected one finished, failed process, got %+v", wait)
	}
	if p := wait.Processes[0]; p.ExitCode == nil || *p.ExitCode != 3 {
		t.Errorf("expected exit code 3, got %v", p.ExitCode)
	}

	// Output can trail the exit by a frame; poll the log rather than trust wait's snapshot.
	var log struct {
		Output string `json:"output"`
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		callTool(t, cs, "yoink_log", map[string]any{"process": "mcp"}, &log)
		if strings.Contains(log.Output, "mcp-windows") {
			return
		}
	}
	t.Errorf("expected process output over MCP, got %q", log.Output)
}
