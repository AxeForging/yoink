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

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get working directory: %v", err)
	}
	return filepath.Dir(wd)
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "yoink-test")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, string(out))
	}
	return bin
}

type testEnv struct {
	bin      string
	sockPath string
	sockDir  string
	t        *testing.T
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	bin := buildBinary(t)
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "yoink.sock")

	env := &testEnv{
		bin:      bin,
		sockPath: sockPath,
		sockDir:  sockDir,
		t:        t,
	}

	// Start daemon
	daemonCmd := exec.Command(bin, "daemon")
	daemonCmd.Env = append(os.Environ(), "YOINK_SOCK="+sockPath, "HOME="+sockDir)
	daemonCmd.Stdout = os.Stderr
	daemonCmd.Stderr = os.Stderr
	if err := daemonCmd.Start(); err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}

	// Wait for socket
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Cleanup(func() {
		// Shutdown daemon via socket
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			json.NewEncoder(conn).Encode(domain.DaemonRequest{Type: domain.ReqShutdown})
			conn.Close()
		}
		daemonCmd.Process.Kill()
		daemonCmd.Wait()
	})

	return env
}

func (e *testEnv) run(args ...string) (string, error) {
	e.t.Helper()
	cmd := exec.Command(e.bin, args...)
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
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	json.NewEncoder(conn).Encode(req)
	var resp domain.DaemonResponse
	json.NewDecoder(conn).Decode(&resp)
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
