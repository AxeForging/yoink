//go:build !windows

package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AxeForging/yoink/domain"
)

// runFrom runs the CLI as if invoked from dir, to exercise behavior that
// depends on the client's own working directory (as opposed to the daemon's).
func (e *testEnv) runFrom(dir string, args ...string) (string, error) {
	e.t.Helper()
	cmd := exec.Command(e.bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "YOINK_SOCK="+e.sockPath, "HOME="+e.sockDir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (e *testEnv) sendDaemonRequest(req domain.DaemonRequest) domain.DaemonResponse {
	e.t.Helper()
	conn, err := net.Dial("unix", e.sockPath)
	if err != nil {
		e.t.Fatalf("cannot connect to daemon: %v", err)
	}
	defer conn.Close()                                //nolint:errcheck
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	json.NewEncoder(conn).Encode(req)                 //nolint:errcheck
	var resp domain.DaemonResponse
	json.NewDecoder(conn).Decode(&resp) //nolint:errcheck
	return resp
}

func TestIntegration_DaemonRunAndList(t *testing.T) {
	env := newTestEnv(t)

	// Run via daemon protocol directly
	runResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo integration-test",
	})
	if !runResp.Success {
		t.Fatalf("run failed: %s", runResp.Error)
	}
	if runResp.ID < 1 {
		t.Errorf("expected positive ID, got %d", runResp.ID)
	}

	time.Sleep(500 * time.Millisecond)

	// List
	listResp := env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqList})
	if !listResp.Success {
		t.Fatalf("list failed: %s", listResp.Error)
	}
	if len(listResp.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(listResp.Processes))
	}
	if listResp.Processes[0].State != domain.StateDone {
		t.Errorf("expected state done, got %s", listResp.Processes[0].State)
	}
}

func TestIntegration_LogOutput(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo hello-integration",
	})

	time.Sleep(500 * time.Millisecond)

	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    1,
		Lines: 10,
	})
	if !logResp.Success {
		t.Fatalf("log failed: %s", logResp.Error)
	}
	if !strings.Contains(logResp.Output, "hello-integration") {
		t.Errorf("expected 'hello-integration' in output, got: %q", logResp.Output)
	}
}

func TestIntegration_SendInput(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "cat",
	})

	time.Sleep(300 * time.Millisecond)

	sendResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqSend,
		ID:    1,
		Input: "integration-input",
	})
	if !sendResp.Success {
		t.Fatalf("send failed: %s", sendResp.Error)
	}

	time.Sleep(500 * time.Millisecond)

	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    1,
		Lines: 10,
	})
	if !strings.Contains(logResp.Output, "integration-input") {
		t.Errorf("expected 'integration-input' in output, got: %q", logResp.Output)
	}
}

func TestIntegration_KillAndClean(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sleep 300",
	})

	time.Sleep(300 * time.Millisecond)

	killResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type: domain.ReqKill,
		ID:   1,
	})
	if !killResp.Success {
		t.Fatalf("kill failed: %s", killResp.Error)
	}

	time.Sleep(500 * time.Millisecond)

	cleanResp := env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqClean})
	if !cleanResp.Success {
		t.Fatalf("clean failed: %s", cleanResp.Error)
	}
	if cleanResp.Cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", cleanResp.Cleaned)
	}

	listResp := env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqList})
	if len(listResp.Processes) != 0 {
		t.Errorf("expected 0 processes after clean, got %d", len(listResp.Processes))
	}
}

func TestIntegration_KillAliasPrintsResolvedID(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sleep 300",
		Alias:   "sleepy",
	})

	time.Sleep(300 * time.Millisecond)

	out, err := env.run("kill", "sleepy")
	if err != nil {
		t.Fatalf("kill by alias failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "process 1") {
		t.Fatalf("expected kill output to include resolved process ID, got: %q", out)
	}
}

func TestIntegration_MultipleProcesses(t *testing.T) {
	env := newTestEnv(t)

	for i := 0; i < 3; i++ {
		resp := env.sendDaemonRequest(domain.DaemonRequest{
			Type:    domain.ReqRun,
			Command: fmt.Sprintf("echo proc-%d", i),
		})
		if !resp.Success {
			t.Fatalf("run %d failed: %s", i, resp.Error)
		}
	}

	time.Sleep(500 * time.Millisecond)

	listResp := env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqList})
	if len(listResp.Processes) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(listResp.Processes))
	}

	// Check all completed
	for _, p := range listResp.Processes {
		if p.State != domain.StateDone {
			t.Errorf("process %d: expected done, got %s", p.ID, p.State)
		}
	}
}

func TestIntegration_WaitForDone(t *testing.T) {
	env := newTestEnv(t)

	// Spawn a fast command
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo wait-done-test",
		Alias:   "fast",
	})

	time.Sleep(500 * time.Millisecond)

	// Wait should succeed immediately (process already done)
	output, err := env.run("wait", "fast", "--timeout", "5", "--poll", "1")
	if err != nil {
		t.Fatalf("wait failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "wait-done-test") {
		t.Errorf("expected output to contain 'wait-done-test', got: %q", output)
	}
}

func TestIntegration_WaitForTimeout(t *testing.T) {
	env := newTestEnv(t)

	// Spawn a long-running command
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sleep 300",
		Alias:   "slow",
	})

	time.Sleep(300 * time.Millisecond)

	// Wait should timeout
	output, err := env.run("wait", "slow", "--timeout", "2", "--poll", "1", "--message", "custom timeout msg")
	if err == nil {
		t.Fatal("expected wait to fail on timeout")
	}
	if !strings.Contains(output, "custom timeout msg") {
		t.Errorf("expected custom timeout message, got: %q", output)
	}
}

func TestIntegration_WaitForFailed(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "exit 7",
	})

	time.Sleep(500 * time.Millisecond)

	// Wait should exit with the process exit code
	_, err := env.run("wait", "1", "--timeout", "5", "--poll", "1")
	if err == nil {
		t.Fatal("expected wait to fail for failed process")
	}
	// The exit code should be 7
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 7 {
			t.Errorf("expected exit code 7, got %d", exitErr.ExitCode())
		}
	}
}

func TestIntegration_WaitStreamsOutput(t *testing.T) {
	env := newTestEnv(t)

	// Spawn a script that outputs lines then exits
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sh -c 'echo line-one; sleep 1; echo line-two; sleep 1; echo line-three'",
		Alias:   "streamer",
	})

	// Wait should capture all output
	output, err := env.run("wait", "streamer", "--timeout", "10", "--poll", "1")
	if err != nil {
		t.Fatalf("wait failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "line-one") {
		t.Errorf("expected 'line-one' in output, got: %q", output)
	}
	if !strings.Contains(output, "line-three") {
		t.Errorf("expected 'line-three' in output, got: %q", output)
	}
}

func TestIntegration_WaitMultipleProcesses(t *testing.T) {
	env := newTestEnv(t)

	// Spawn two fast commands
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo multi-a",
		Alias:   "a",
	})
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo multi-b",
		Alias:   "b",
	})

	time.Sleep(500 * time.Millisecond)

	// Wait for both
	output, err := env.run("wait", "a", "b", "--timeout", "5", "--poll", "1")
	if err != nil {
		t.Fatalf("wait failed: %v\noutput: %s", err, output)
	}
	// Multi-wait prefixes output with [alias]
	if !strings.Contains(output, "[a]") {
		t.Errorf("expected [a] label in output, got: %q", output)
	}
	if !strings.Contains(output, "[b]") {
		t.Errorf("expected [b] label in output, got: %q", output)
	}
	if !strings.Contains(output, "multi-a") {
		t.Errorf("expected 'multi-a' in output, got: %q", output)
	}
	if !strings.Contains(output, "multi-b") {
		t.Errorf("expected 'multi-b' in output, got: %q", output)
	}
}

func TestIntegration_WaitMultipleFailFast(t *testing.T) {
	env := newTestEnv(t)

	// One succeeds, one fails
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo good",
		Alias:   "ok",
	})
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "exit 5",
		Alias:   "bad",
	})

	time.Sleep(500 * time.Millisecond)

	_, err := env.run("wait", "ok", "bad", "--timeout", "5", "--poll", "1")
	if err == nil {
		t.Fatal("expected wait to fail when one process fails")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 5 {
			t.Errorf("expected exit code 5, got %d", exitErr.ExitCode())
		}
	}
}

func TestIntegration_ProcessExitCode(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "exit 7",
	})

	time.Sleep(500 * time.Millisecond)

	listResp := env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqList})
	if len(listResp.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(listResp.Processes))
	}
	p := listResp.Processes[0]
	if p.State != domain.StateFailed {
		t.Errorf("expected state failed, got %s", p.State)
	}
	if p.ExitCode != 7 {
		t.Errorf("expected exit code 7, got %d", p.ExitCode)
	}
}

func TestIntegration_SendTypeMode(t *testing.T) {
	env := newTestEnv(t)

	// Spawn cat which echoes back what it receives.
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "cat",
		Alias:   "typer",
	})

	time.Sleep(300 * time.Millisecond)

	// Use the CLI --type flag with a short delay so the test stays fast.
	out, err := env.run("send", "typer", "type-mode-works", "--type", "--delay", "5")
	if err != nil {
		t.Fatalf("send --type failed: %v\noutput: %s", err, out)
	}

	time.Sleep(500 * time.Millisecond)

	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    1,
		Lines: 20,
	})
	if !strings.Contains(logResp.Output, "type-mode-works") {
		t.Errorf("expected 'type-mode-works' in output, got: %q", logResp.Output)
	}
}

func TestIntegration_SendNoEnter(t *testing.T) {
	env := newTestEnv(t)

	// Script reads a line and echoes it back prefixed with "got-".
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sh -c 'read -r line; echo got-$line'",
		Alias:   "reader",
	})

	time.Sleep(300 * time.Millisecond)

	// Send partial input without newline via --no-enter.
	_, err := env.run("send", "reader", "hello", "--no-enter")
	if err != nil {
		t.Fatalf("send --no-enter failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Process should not have produced "got-" yet.
	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    1,
		Lines: 20,
	})
	if strings.Contains(logResp.Output, "got-") {
		t.Errorf("did not expect 'got-' before newline was sent, got: %q", logResp.Output)
	}

	// Now complete with just a newline.
	_, err = env.run("send", "reader", "")
	if err != nil {
		t.Fatalf("send (newline completion) failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	logResp = env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    1,
		Lines: 20,
	})
	if !strings.Contains(logResp.Output, "got-hello") {
		t.Errorf("expected 'got-hello' after newline, got: %q", logResp.Output)
	}
}

func TestIntegration_SendSubmitUsesCarriageReturn(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sh -c 'stty raw -echo; dd bs=1 count=2 2>/dev/null | od -An -tx1; sleep 1'",
		Alias:   "raw-reader",
	})

	time.Sleep(300 * time.Millisecond)

	out, err := env.run("send", "raw-reader", "x", "--submit")
	if err != nil {
		t.Fatalf("send --submit failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "process 1") {
		t.Fatalf("expected send output to include resolved process ID, got: %q", out)
	}

	time.Sleep(500 * time.Millisecond)

	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		Alias: "raw-reader",
		Lines: 20,
	})
	if !strings.Contains(logResp.Output, "78 0d") {
		t.Errorf("expected --submit to send CR after text, got: %q", logResp.Output)
	}
}

func TestIntegration_Snapshot_PlainProcess(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sh -c 'printf \"\\033[2J\\033[3;5Hsnap-integration-test\"; sleep 300'",
		Alias:   "tui",
	})

	time.Sleep(400 * time.Millisecond)

	out, err := env.run("snapshot", "tui")
	if err != nil {
		t.Fatalf("snapshot failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "snap-integration-test") {
		t.Errorf("expected 'snap-integration-test' in snapshot, got: %q", out)
	}
}

func TestIntegration_Snapshot_ByAlias(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sh -c 'echo alias-snap-check; sleep 300'",
		Alias:   "checker",
	})

	time.Sleep(400 * time.Millisecond)

	out, err := env.run("snap", "checker") // test the "snap" alias
	if err != nil {
		t.Fatalf("snap alias failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "alias-snap-check") {
		t.Errorf("expected 'alias-snap-check' in snapshot, got: %q", out)
	}
}

func TestIntegration_Snapshot_TopCommand(t *testing.T) {
	if _, err := exec.LookPath("top"); err != nil {
		t.Skip("top not available on this system")
	}

	env := newTestEnv(t)

	// Limit to 18 lines so output stays within the 24-row VT buffer regardless
	// of how many processes are running. Works on Linux and macOS.
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sh -c 'top -b -n 1 2>/dev/null | head -18 || top -l 1 2>/dev/null | head -18'",
		Alias:   "top",
	})

	// Wait for top to finish its single batch iteration.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		listResp := env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqList})
		for _, p := range listResp.Processes {
			if p.Alias == "top" && p.State == domain.StateDone {
				goto ready
			}
		}
	}
	t.Fatal("timed out waiting for top to finish")
ready:

	out, err := env.run("snapshot", "top")
	if err != nil {
		t.Fatalf("snapshot of top failed: %v\noutput: %s", err, out)
	}

	// top output always contains these fields regardless of platform
	for _, want := range []string{"PID", "%"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in top snapshot, got: %q", want, out)
		}
	}

	// Prove the value: log would give raw escape garbage, snapshot gives clean text.
	// Verify there are no raw CSI escape sequences in the snapshot output.
	if strings.Contains(out, "\x1b[") {
		t.Errorf("snapshot should not contain raw ANSI escape sequences, got: %q", out)
	}
}

func TestIntegration_RunUsesClientCwdNotDaemonCwd(t *testing.T) {
	env := newTestEnv(t)

	// The daemon process itself inherits this test binary's own cwd (its Dir
	// is never set in newTestEnv) — capture it so we can prove the spawned
	// process did NOT fall back to it.
	daemonCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get test working directory: %v", err)
	}

	clientDir := t.TempDir()
	wantDir, err := filepath.EvalSymlinks(clientDir)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}
	if wantDir == daemonCwd {
		t.Fatalf("test setup invalid: client dir %q must differ from daemon cwd", wantDir)
	}

	out, err := env.runFrom(clientDir, "run", "--alias", "pwdcheck", "pwd")
	if err != nil {
		t.Fatalf("run failed: %v\noutput: %s", err, out)
	}

	time.Sleep(500 * time.Millisecond)

	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		Alias: "pwdcheck",
		Lines: 10,
	})
	if !strings.Contains(logResp.Output, wantDir) {
		t.Errorf("expected process to run in the client's cwd %q, got: %q", wantDir, logResp.Output)
	}
	if strings.Contains(logResp.Output, daemonCwd) {
		t.Errorf("process ran in the daemon's cwd %q instead of the client's, got: %q", daemonCwd, logResp.Output)
	}
}

func TestIntegration_RunCwdFlagOverridesClientCwd(t *testing.T) {
	env := newTestEnv(t)

	explicitDir := t.TempDir()
	wantDir, err := filepath.EvalSymlinks(explicitDir)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}
	callerDir := t.TempDir()

	out, err := env.runFrom(callerDir, "run", "--alias", "explicit-cwd", "--cwd", explicitDir, "pwd")
	if err != nil {
		t.Fatalf("run --cwd failed: %v\noutput: %s", err, out)
	}

	time.Sleep(500 * time.Millisecond)

	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		Alias: "explicit-cwd",
		Lines: 10,
	})
	if !strings.Contains(logResp.Output, wantDir) {
		t.Errorf("expected --cwd to override the caller's directory, got: %q", logResp.Output)
	}
}

func TestIntegration_RunCwdFlagResolvesRelativeToCaller(t *testing.T) {
	env := newTestEnv(t)

	callerDir := t.TempDir()
	subDir := filepath.Join(callerDir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	wantDir, err := filepath.EvalSymlinks(subDir)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}

	out, err := env.runFrom(callerDir, "run", "--alias", "relative-cwd", "--cwd", "sub", "pwd")
	if err != nil {
		t.Fatalf("run --cwd sub failed: %v\noutput: %s", err, out)
	}

	time.Sleep(500 * time.Millisecond)

	logResp := env.sendDaemonRequest(domain.DaemonRequest{
		Type:  domain.ReqLog,
		Alias: "relative-cwd",
		Lines: 10,
	})
	if !strings.Contains(logResp.Output, wantDir) {
		t.Errorf("expected a relative --cwd to resolve against the caller's directory %q, got: %q", callerDir, logResp.Output)
	}
}

func TestIntegration_RunLogFileTeesFullOutput(t *testing.T) {
	env := newTestEnv(t)

	logPath := filepath.Join(t.TempDir(), "full-output.log")

	out, err := env.run("run", "--alias", "teed", "--log-file", logPath,
		"sh -c 'echo teed-line-one; echo teed-line-two'")
	if err != nil {
		t.Fatalf("run --log-file failed: %v\noutput: %s", err, out)
	}

	time.Sleep(500 * time.Millisecond)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "teed-line-one") || !strings.Contains(content, "teed-line-two") {
		t.Errorf("expected log file to contain the full process output, got: %q", content)
	}
}

func TestIntegration_LsFullShowsUntruncatedCommandAndCwd(t *testing.T) {
	env := newTestEnv(t)

	longArg := strings.Repeat("x", 80)
	longCommand := "echo " + longArg

	if _, err := env.run("run", "--alias", "longcmd", longCommand); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Default ls must keep truncating — this is the regression guard that
	// --full must not change existing behavior.
	defaultOut, err := env.run("ls")
	if err != nil {
		t.Fatalf("ls failed: %v\noutput: %s", err, defaultOut)
	}
	if !strings.Contains(defaultOut, "...") {
		t.Errorf("expected default ls to truncate a long command, got: %q", defaultOut)
	}
	if strings.Contains(defaultOut, longCommand) {
		t.Errorf("expected default ls NOT to contain the full untruncated command, got: %q", defaultOut)
	}

	fullOut, err := env.run("ls", "--full")
	if err != nil {
		t.Fatalf("ls --full failed: %v\noutput: %s", err, fullOut)
	}
	if !strings.Contains(fullOut, longCommand) {
		t.Errorf("expected ls --full to contain the untruncated command, got: %q", fullOut)
	}
	if !strings.Contains(fullOut, "CWD") {
		t.Errorf("expected ls --full header to include a CWD column, got: %q", fullOut)
	}
}

func TestIntegration_CleanAll(t *testing.T) {
	env := newTestEnv(t)

	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo clean-all-done",
	})
	env.sendDaemonRequest(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sleep 300",
	})

	time.Sleep(500 * time.Millisecond)

	// Plain clean must not touch the still-running process.
	out, err := env.run("clean")
	if err != nil {
		t.Fatalf("clean failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Cleaned 1 process") {
		t.Errorf("expected plain clean to report removing only the finished process, got: %q", out)
	}

	listResp := env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqList})
	if len(listResp.Processes) != 1 || listResp.Processes[0].State != domain.StateRunning {
		t.Fatalf("expected the running process to survive plain clean, got: %+v", listResp.Processes)
	}

	// clean --all must stop and remove the still-running process too.
	out, err = env.run("clean", "--all")
	if err != nil {
		t.Fatalf("clean --all failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Stopped and removed 1 process") {
		t.Errorf("expected clean --all to report stopping the running process, got: %q", out)
	}

	listResp = env.sendDaemonRequest(domain.DaemonRequest{Type: domain.ReqList})
	if len(listResp.Processes) != 0 {
		t.Errorf("expected no processes left after clean --all, got %d", len(listResp.Processes))
	}
}
