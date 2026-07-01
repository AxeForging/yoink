//go:build !windows

package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AxeForging/yoink/domain"
)

func waitForOutput(t *testing.T, pm *ProcessManager, id int, expected string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, _ := pm.GetLogEntries(id, 1000)
		for _, e := range entries {
			if strings.Contains(e.Text, expected) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	output, _ := pm.GetOutput(id, 1000)
	t.Fatalf("timed out waiting for %q in output, got: %q", expected, output)
}

func waitForState(t *testing.T, pm *ProcessManager, id int, state domain.ProcessState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, p := range pm.List() {
			if p.ID == id && p.State == state {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for process %d to reach state %s", id, state)
}

func TestProcessManager_SpawnEcho(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("echo hello-yoink", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if id != 1 {
		t.Errorf("expected ID 1, got %d", id)
	}

	waitForOutput(t, pm, id, "hello-yoink", 5*time.Second)
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	// Check exit code
	for _, p := range pm.List() {
		if p.ID == id {
			if p.ExitCode != 0 {
				t.Errorf("expected exit code 0, got %d", p.ExitCode)
			}
			return
		}
	}
	t.Error("process not found in list")
}

func TestProcessManager_SpawnAndSendInput(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// cat will echo back what we send via PTY
	id, err := pm.Spawn("cat", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond) // let cat start

	if err := pm.SendInput(id, "test-input-123", SendOptions{}); err != nil {
		t.Fatalf("SendInput failed: %v", err)
	}

	waitForOutput(t, pm, id, "test-input-123", 5*time.Second)
}

func TestProcessManager_SpawnFailedExitCode(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("exit 42", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	waitForState(t, pm, id, domain.StateFailed, 5*time.Second)

	for _, p := range pm.List() {
		if p.ID == id {
			if p.ExitCode != 42 {
				t.Errorf("expected exit code 42, got %d", p.ExitCode)
			}
			return
		}
	}
	t.Error("process not found in list")
}

func TestProcessManager_Kill(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("sleep 300", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := pm.Kill(id, false); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}

	waitForState(t, pm, id, domain.StateFailed, 5*time.Second)
}

func TestProcessManager_KillForce(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("sleep 300", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := pm.Kill(id, true); err != nil {
		t.Fatalf("Kill (force) failed: %v", err)
	}

	waitForState(t, pm, id, domain.StateFailed, 5*time.Second)
}

func TestProcessManager_List(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	pm.Spawn("echo one", "", SpawnOptions{})
	pm.Spawn("echo two", "", SpawnOptions{})
	pm.Spawn("sleep 300", "", SpawnOptions{})

	time.Sleep(500 * time.Millisecond)

	list := pm.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(list))
	}
}

func TestProcessManager_Clean(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	pm.Spawn("echo done-quick", "", SpawnOptions{})
	id2, _ := pm.Spawn("sleep 300", "", SpawnOptions{})

	// Wait for echo to finish
	time.Sleep(500 * time.Millisecond)

	cleaned := pm.Clean()
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", cleaned)
	}

	list := pm.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 remaining process, got %d", len(list))
	}
	if list[0].ID != id2 {
		t.Errorf("expected remaining process ID %d, got %d", id2, list[0].ID)
	}
}

func TestProcessManager_GetOutput_NotFound(t *testing.T) {
	pm := NewProcessManager(100)
	_, err := pm.GetOutput(999, 10)
	if err == nil {
		t.Error("expected error for non-existent process")
	}
}

func TestProcessManager_SendInput_NotRunning(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, _ := pm.Spawn("echo fast", "", SpawnOptions{})
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	err := pm.SendInput(id, "too late", SendOptions{})
	if err == nil {
		t.Error("expected error sending to finished process")
	}
}

func TestProcessManager_Kill_NotFound(t *testing.T) {
	pm := NewProcessManager(100)
	err := pm.Kill(999, false)
	if err == nil {
		t.Error("expected error for non-existent process")
	}
}

func TestProcessManager_MultilineOutput(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("printf 'line1\\nline2\\nline3\\n'", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	output, err := pm.GetOutput(id, 10)
	if err != nil {
		t.Fatalf("GetOutput failed: %v", err)
	}

	if !strings.Contains(output, "line1") || !strings.Contains(output, "line3") {
		t.Errorf("expected multiline output, got: %q", output)
	}
}

func TestProcessManager_LabeledOutput(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("cat", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	pm.SendInput(id, "hello", SendOptions{})
	time.Sleep(500 * time.Millisecond)

	output, _ := pm.GetOutput(id, 100)
	if !strings.Contains(output, "[in] hello") {
		t.Errorf("expected labeled stdin in output, got: %q", output)
	}
	if !strings.Contains(output, "[out]") {
		t.Errorf("expected labeled stdout in output, got: %q", output)
	}
}

func TestProcessManager_SendRedacted(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("cat", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	pm.SendInput(id, "super-secret-token", SendOptions{Redacted: true})
	time.Sleep(500 * time.Millisecond)

	// Formatted output should show redacted
	output, _ := pm.GetOutput(id, 100)
	if strings.Contains(output, "super-secret-token") {
		t.Errorf("redacted input should NOT appear in formatted output, got: %q", output)
	}
	if !strings.Contains(output, "********") {
		t.Errorf("expected '********' for redacted input in output, got: %q", output)
	}
	if !strings.Contains(output, "[in] ********") {
		t.Errorf("expected '[in] ********' for redacted input, got: %q", output)
	}

	// Raw entries should still have the text (for daemon internal use) but marked redacted
	entries, _ := pm.GetLogEntries(id, 100)
	found := false
	for _, e := range entries {
		if e.Redacted && e.Source == domain.SourceStdin {
			found = true
			if e.Text != "super-secret-token" {
				t.Errorf("raw entry text should contain actual input, got: %q", e.Text)
			}
		}
	}
	if !found {
		t.Error("expected to find a redacted stdin entry")
	}
}

func TestProcessManager_GetNewOutput(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// Use a script that produces distinct reply lines so we can wait for exact output
	id, err := pm.Spawn("sh -c 'while read line; do echo reply-$line; done'", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	pm.SendInput(id, "first", SendOptions{})
	waitForOutput(t, pm, id, "reply-first", 5*time.Second)
	// Let PTY echo settle
	time.Sleep(200 * time.Millisecond)

	// First --new call: returns everything so far
	output, err := pm.GetNewOutput(id)
	if err != nil {
		t.Fatalf("GetNewOutput failed: %v", err)
	}
	if !strings.Contains(output, "reply-first") {
		t.Errorf("expected 'reply-first' in output, got: %q", output)
	}

	// Second --new call: no new output
	output, err = pm.GetNewOutput(id)
	if err != nil {
		t.Fatalf("GetNewOutput failed: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output, got: %q", output)
	}

	// Write more
	pm.SendInput(id, "second", SendOptions{})
	waitForOutput(t, pm, id, "reply-second", 5*time.Second)
	time.Sleep(200 * time.Millisecond)

	// Third --new call: only new lines
	output, err = pm.GetNewOutput(id)
	if err != nil {
		t.Fatalf("GetNewOutput failed: %v", err)
	}
	if !strings.Contains(output, "reply-second") {
		t.Errorf("expected 'reply-second' in output, got: %q", output)
	}
	if strings.Contains(output, "reply-first") {
		t.Errorf("did not expect 'reply-first' in new output, got: %q", output)
	}
}

func TestProcessManager_GetNewOutput_NotFound(t *testing.T) {
	pm := NewProcessManager(100)
	_, err := pm.GetNewOutput(999)
	if err == nil {
		t.Error("expected error for non-existent process")
	}
}

func TestProcessManager_SpawnWithAlias(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("echo aliased", "deploy", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	// Verify alias is stored in process info
	for _, p := range pm.List() {
		if p.ID == id {
			if p.Alias != "deploy" {
				t.Errorf("expected alias 'deploy', got %q", p.Alias)
			}
			return
		}
	}
	t.Error("process not found in list")
}

func TestProcessManager_ResolveID_ByAlias(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("sleep 300", "myproc", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	resolved, err := pm.ResolveID("myproc")
	if err != nil {
		t.Fatalf("ResolveID failed: %v", err)
	}
	if resolved != id {
		t.Errorf("expected resolved ID %d, got %d", id, resolved)
	}
}

func TestProcessManager_ResolveID_ByNumericID(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	pm.Spawn("sleep 300", "", SpawnOptions{})

	resolved, err := pm.ResolveID("1")
	if err != nil {
		t.Fatalf("ResolveID failed: %v", err)
	}
	if resolved != 1 {
		t.Errorf("expected resolved ID 1, got %d", resolved)
	}
}

func TestProcessManager_ResolveID_NotFound(t *testing.T) {
	pm := NewProcessManager(100)
	_, err := pm.ResolveID("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent alias")
	}
}

func TestProcessManager_DuplicateAlias(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	_, err := pm.Spawn("sleep 300", "dup", SpawnOptions{})
	if err != nil {
		t.Fatalf("first Spawn failed: %v", err)
	}

	_, err = pm.Spawn("sleep 300", "dup", SpawnOptions{})
	if err == nil {
		t.Error("expected error for duplicate alias")
	}
}

func TestProcessManager_AliasReuseAfterClean(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, _ := pm.Spawn("echo fast", "reuse-me", SpawnOptions{})
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	pm.Clean()

	// Alias should be available again
	id2, err := pm.Spawn("echo again", "reuse-me", SpawnOptions{})
	if err != nil {
		t.Fatalf("expected alias reuse after clean, got: %v", err)
	}
	if id2 == id {
		t.Error("expected different ID after clean")
	}
}

func TestProcessManager_AliasWithOperations(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, _ := pm.Spawn("cat", "interactive", SpawnOptions{})
	time.Sleep(200 * time.Millisecond)

	// Resolve alias and use for operations
	resolved, _ := pm.ResolveID("interactive")
	if err := pm.SendInput(resolved, "hello-alias", SendOptions{}); err != nil {
		t.Fatalf("SendInput via alias failed: %v", err)
	}

	waitForOutput(t, pm, id, "hello-alias", 5*time.Second)

	// Kill via resolved alias
	if err := pm.Kill(resolved, false); err != nil {
		t.Fatalf("Kill via alias failed: %v", err)
	}
	waitForState(t, pm, id, domain.StateFailed, 5*time.Second)
}

// --- Type mode tests ---

func TestProcessManager_TypeMode_OutputAppears(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// cat echoes back what it receives; with type mode the chars arrive one by one
	// but the output should still contain the full string once newline is sent.
	id, err := pm.Spawn("cat", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := pm.SendInput(id, "typed-hello", SendOptions{TypeMode: true, TypeDelay: 5 * time.Millisecond}); err != nil {
		t.Fatalf("SendInput (type mode) failed: %v", err)
	}

	waitForOutput(t, pm, id, "typed-hello", 5*time.Second)
}

func TestProcessManager_TypeMode_DelayIsRespected(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("cat", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// 5 chars × 20ms = at least 100ms elapsed
	input := "abcde"
	delay := 20 * time.Millisecond
	start := time.Now()
	if err := pm.SendInput(id, input, SendOptions{TypeMode: true, TypeDelay: delay}); err != nil {
		t.Fatalf("SendInput (type mode with delay) failed: %v", err)
	}
	elapsed := time.Since(start)

	minExpected := time.Duration(len([]rune(input))-1) * delay
	if elapsed < minExpected {
		t.Errorf("expected at least %v elapsed for type mode, got %v", minExpected, elapsed)
	}

	waitForOutput(t, pm, id, input, 5*time.Second)
}

func TestProcessManager_TypeMode_ZeroDelay(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("cat", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Zero delay: chars sent immediately, no sleep
	if err := pm.SendInput(id, "nodelay", SendOptions{TypeMode: true, TypeDelay: 0}); err != nil {
		t.Fatalf("SendInput (type mode, zero delay) failed: %v", err)
	}

	waitForOutput(t, pm, id, "nodelay", 5*time.Second)
}

func TestProcessManager_NoEnter_NoLineEcho(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// Use a shell that echoes lines only after receiving a newline.
	// send-without-enter should NOT produce a completed line in output.
	id, err := pm.Spawn("sh -c 'read -r line; echo got-$line'", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Send partial input without newline — the read builtin won't complete yet.
	if err := pm.SendInput(id, "partial", SendOptions{NoEnter: true}); err != nil {
		t.Fatalf("SendInput (no-enter) failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// No "got-" line should have appeared yet because read is still waiting.
	entries, _ := pm.GetLogEntries(id, 100)
	for _, e := range entries {
		if e.Source == domain.SourceStdout && strings.Contains(e.Text, "got-") {
			t.Errorf("did not expect 'got-' output before newline, got: %q", e.Text)
		}
	}

	// Now complete with newline; process should echo got-partial.
	if err := pm.SendInput(id, "", SendOptions{}); err != nil {
		t.Fatalf("SendInput (newline completion) failed: %v", err)
	}

	waitForOutput(t, pm, id, "got-partial", 5*time.Second)
}

func TestProcessManager_SubmitUsesCarriageReturn(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("sh -c 'stty raw -echo; dd bs=1 count=2 2>/dev/null | od -An -tx1; sleep 1'", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := pm.SendInput(id, "x", SendOptions{Submit: true}); err != nil {
		t.Fatalf("SendInput (submit) failed: %v", err)
	}

	waitForOutput(t, pm, id, "78 0d", 5*time.Second)
}

func TestProcessManager_TypeMode_WithRedaction(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("cat", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := pm.SendInput(id, "secret-typed", SendOptions{TypeMode: true, TypeDelay: 5 * time.Millisecond, Redacted: true}); err != nil {
		t.Fatalf("SendInput (type mode + redacted) failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	output, _ := pm.GetOutput(id, 100)
	if strings.Contains(output, "secret-typed") {
		t.Errorf("redacted input should NOT appear in formatted output, got: %q", output)
	}
	if !strings.Contains(output, "********") {
		t.Errorf("expected '********' for redacted input in output, got: %q", output)
	}
}

func TestProcessManager_TypeMode_NoEnter_Combined(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// Type char by char without a trailing newline, then complete separately.
	id, err := pm.Spawn("sh -c 'read -r line; echo got-$line'", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := pm.SendInput(id, "combo", SendOptions{TypeMode: true, TypeDelay: 5 * time.Millisecond, NoEnter: true}); err != nil {
		t.Fatalf("SendInput (type + no-enter) failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Still no completed line.
	entries, _ := pm.GetLogEntries(id, 100)
	for _, e := range entries {
		if e.Source == domain.SourceStdout && strings.Contains(e.Text, "got-") {
			t.Errorf("did not expect 'got-' before newline, got: %q", e.Text)
		}
	}

	// Complete with newline.
	if err := pm.SendInput(id, "", SendOptions{}); err != nil {
		t.Fatalf("SendInput (newline) failed: %v", err)
	}

	waitForOutput(t, pm, id, "got-combo", 5*time.Second)
}

// --- Snapshot tests ---

func TestProcessManager_Snapshot_PlainText(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// A plain process with no TUI — snapshot should still show its output.
	id, err := pm.Spawn("sh -c 'echo hello-snap; sleep 300'", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	waitForOutput(t, pm, id, "hello-snap", 5*time.Second)

	snap, err := pm.Snapshot(id)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !strings.Contains(snap, "hello-snap") {
		t.Errorf("expected 'hello-snap' in snapshot, got:\n%s", snap)
	}
}

func TestProcessManager_Snapshot_PositionedText(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// Use ANSI cursor positioning — yoink log would show raw escape garbage,
	// but snapshot should give clean rendered text at the correct coordinates.
	id, err := pm.Spawn(`sh -c 'printf "\033[2J\033[3;5Hyoink-tui-test\033[5;1Hsecond-line"; sleep 300'`, "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	snap, err := pm.Snapshot(id)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !strings.Contains(snap, "yoink-tui-test") {
		t.Errorf("expected 'yoink-tui-test' at row 3 in snapshot, got:\n%s", snap)
	}
	if !strings.Contains(snap, "second-line") {
		t.Errorf("expected 'second-line' at row 5 in snapshot, got:\n%s", snap)
	}

	// Verify positioning: "yoink-tui-test" should appear before "second-line"
	lines := strings.Split(snap, "\n")
	row3, row5 := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "yoink-tui-test") {
			row3 = i
		}
		if strings.Contains(line, "second-line") {
			row5 = i
		}
	}
	if row3 < 0 || row5 < 0 || row3 >= row5 {
		t.Errorf("expected 'yoink-tui-test' (row %d) before 'second-line' (row %d)", row3, row5)
	}
}

func TestProcessManager_Snapshot_ClearAndRedraw(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// Process that draws one thing, then clears and draws something else.
	// Snapshot should reflect the FINAL screen state, not the history.
	// yoink log would show both; snapshot shows only what's currently rendered.
	id, err := pm.Spawn(`sh -c 'printf "initial-content"; sleep 0.2; printf "\033[2Jfinal-content"; sleep 300'`, "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(600 * time.Millisecond)

	snap, err := pm.Snapshot(id)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !strings.Contains(snap, "final-content") {
		t.Errorf("expected 'final-content' in snapshot, got:\n%s", snap)
	}
	if strings.Contains(snap, "initial-content") {
		t.Errorf("expected 'initial-content' to be cleared from snapshot, got:\n%s", snap)
	}
}

func TestProcessManager_Snapshot_FinishedProcess(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// Snapshot works on finished processes — the VT holds the last frame.
	id, err := pm.Spawn("echo snap-after-exit", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	snap, err := pm.Snapshot(id)
	if err != nil {
		t.Fatalf("Snapshot of finished process failed: %v", err)
	}
	if !strings.Contains(snap, "snap-after-exit") {
		t.Errorf("expected 'snap-after-exit' in snapshot of finished process, got:\n%s", snap)
	}
}

func TestProcessManager_Snapshot_NotFound(t *testing.T) {
	pm := NewProcessManager(100)
	_, err := pm.Snapshot(999)
	if err == nil {
		t.Error("expected error for non-existent process")
	}
}

func TestProcessManager_Snapshot_ScreenDimensions(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// VT emulator is 80×24. Content placed beyond col 80 or row 24 is clipped.
	// Content within bounds must be reachable via snapshot.
	id, err := pm.Spawn(`sh -c 'printf "\033[24;1Hlast-row"; sleep 300'`, "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	snap, err := pm.Snapshot(id)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if !strings.Contains(snap, "last-row") {
		t.Errorf("expected text at last row (row 24) in snapshot, got:\n%s", snap)
	}
}

func TestProcessManager_Snapshot_TopCommand(t *testing.T) {
	// top is the canonical TUI process — log gives garbage, snapshot gives clean text.
	if _, err := exec.LookPath("top"); err != nil {
		t.Skip("top not available on this system")
	}

	pm := NewProcessManager(100)
	defer pm.Shutdown()

	// Limit to 18 lines so output fits within the 24-row VT buffer regardless
	// of how many processes are running. Works on both Linux (top -b -n 1) and
	// macOS (top -l 1). The column-header row (with "PID") is always within
	// the first 10 lines of top output, well inside the 18-line window.
	id, err := pm.Spawn("sh -c 'top -b -n 1 2>/dev/null | head -18 || top -l 1 2>/dev/null | head -18'", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	waitForState(t, pm, id, domain.StateDone, 10*time.Second)

	snap, err := pm.Snapshot(id)
	if err != nil {
		t.Fatalf("Snapshot of top failed: %v", err)
	}

	// top output always contains these header fields within the first 18 lines
	for _, want := range []string{"PID", "%"} {
		if !strings.Contains(snap, want) {
			t.Errorf("expected %q in top snapshot, got:\n%s", want, snap)
		}
	}

	// Key value: no raw ANSI escape sequences in snapshot output
	if strings.Contains(snap, "\x1b[") {
		t.Errorf("snapshot must not contain raw ANSI escape sequences, got:\n%s", snap)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "color codes stripped",
			input: "\x1b[31mred\x1b[0m",
			want:  "red",
		},
		{
			name:  "hide cursor stripped (terminal corruption prevention)",
			input: "\x1b[?25l",
			want:  "",
		},
		{
			name:  "show cursor stripped",
			input: "\x1b[?25h",
			want:  "",
		},
		{
			name:  "cursor position stripped",
			input: "\x1b[1;1H",
			want:  "",
		},
		{
			name:  "erase display stripped",
			input: "\x1b[2J",
			want:  "",
		},
		{
			name:  "OSC title sequence stripped",
			input: "\x1b]0;title\x07rest",
			want:  "rest",
		},
		{
			name:  "OSC with string terminator stripped",
			input: "\x1b]2;title\x1b\\rest",
			want:  "rest",
		},
		{
			name:  "other two-byte ESC sequence stripped",
			input: "\x1b=text",
			want:  "text",
		},
		{
			name:  "text preserved between stripped sequences",
			input: "\x1b[31mhello\x1b[0m world",
			want:  "hello world",
		},
		{
			name:  "alternate screen escape stripped",
			input: "\x1b[?1049htext\x1b[?1049l",
			want:  "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripVTProblematic(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "plain text passes through",
			input: []byte("hello world"),
			want:  []byte("hello world"),
		},
		{
			name:  "normal CSI erase screen passes through",
			input: []byte("\x1b[2J"),
			want:  []byte("\x1b[2J"),
		},
		{
			name:  "normal CSI color passes through",
			input: []byte("\x1b[31mred\x1b[0m"),
			want:  []byte("\x1b[31mred\x1b[0m"),
		},
		{
			name:  "normal CSI cursor move passes through",
			input: []byte("\x1b[1;1H"),
			want:  []byte("\x1b[1;1H"),
		},
		{
			// kitty keyboard protocol: CSI > 1 u  (enable keyboard enhancement)
			name:  "CSI > 1 u stripped",
			input: []byte("\x1b[>1u"),
			want:  []byte{},
		},
		{
			// kitty keyboard protocol: CSI < u  (disable keyboard enhancement)
			name:  "CSI < u stripped",
			input: []byte("\x1b[<u"),
			want:  []byte{},
		},
		{
			// kitty keyboard: CSI > 4;2 m  (modifyOtherKeys)
			name:  "CSI > 4;2 m stripped",
			input: []byte("\x1b[>4;2m"),
			want:  []byte{},
		},
		{
			// kitty keyboard: CSI = 1 u  (push flags)
			name:  "CSI = 1 u stripped",
			input: []byte("\x1b[=1u"),
			want:  []byte{},
		},
		{
			// kitty sequence surrounded by normal output
			name:  "kitty sequence stripped from surrounding text",
			input: []byte("before\x1b[>1uafter"),
			want:  []byte("beforeafter"),
		},
		{
			// multiple problematic sequences interleaved with normal ones
			name:  "multiple sequences mixed",
			input: []byte("\x1b[2J\x1b[>1u\x1b[H\x1b[<u\x1b[31m"),
			want:  []byte("\x1b[2J\x1b[H\x1b[31m"),
		},
		{
			name:  "empty input",
			input: []byte{},
			want:  []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripVTProblematic(tt.input)
			if string(got) != string(tt.want) {
				t.Errorf("stripVTProblematic(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessManager_Spawn_UsesGivenCwd(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	dir := t.TempDir()
	// Resolve symlinks (e.g. macOS /tmp -> /private/tmp) so this matches
	// exactly what the spawned shell's `pwd` reports.
	wantDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}

	id, err := pm.Spawn("pwd", "", SpawnOptions{Cwd: dir})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	waitForOutput(t, pm, id, wantDir, 5*time.Second)

	for _, p := range pm.List() {
		if p.ID == id {
			if p.Cwd != dir {
				t.Errorf("expected ProcessInfo.Cwd %q, got %q", dir, p.Cwd)
			}
			return
		}
	}
	t.Error("process not found in list")
}

func TestProcessManager_Spawn_DefaultCwdIsUnset(t *testing.T) {
	// Regression guard: without an explicit SpawnOptions.Cwd, ProcessInfo.Cwd
	// must stay empty rather than defaulting to some resolved path — callers
	// (like ls --full) rely on empty meaning "inherited daemon cwd".
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("echo no-cwd-set", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	for _, p := range pm.List() {
		if p.ID == id {
			if p.Cwd != "" {
				t.Errorf("expected empty Cwd when none was requested, got %q", p.Cwd)
			}
			return
		}
	}
	t.Error("process not found in list")
}

func TestProcessManager_Spawn_LogFileCapturesOutputBeyondRingBufferLimit(t *testing.T) {
	// The whole point of --log-file is to escape the ring buffer's size cap,
	// so use a tiny buffer here to prove the file keeps everything the
	// buffer necessarily discards.
	pm := NewProcessManager(3)
	defer pm.Shutdown()

	logPath := filepath.Join(t.TempDir(), "full.log")

	id, err := pm.Spawn(`sh -c 'for i in $(seq 1 20); do echo "line-$i"; done'`, "", SpawnOptions{LogFile: logPath})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	entries, err := pm.GetLogEntries(id, 100)
	if err != nil {
		t.Fatalf("GetLogEntries failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected the ring buffer capped at 3 entries, got %d: %+v", len(entries), entries)
	}
	if entries[len(entries)-1].Text != "line-20" {
		t.Errorf("expected ring buffer to retain only the most recent lines, got %+v", entries)
	}

	// Give the capture goroutine a moment to close the file after process exit.
	deadline := time.Now().Add(2 * time.Second)
	var content string
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}
		content = string(data)
		if strings.Count(content, "\n") >= 20 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	for i := 1; i <= 20; i++ {
		want := fmt.Sprintf("line-%d", i)
		if !strings.Contains(content, want) {
			t.Fatalf("expected log file to contain %q (buffer only kept the last 3), got:\n%s", want, content)
		}
	}
}

func TestProcessManager_Spawn_LogFileOpenFailureLeavesNoProcessRegistered(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	badPath := filepath.Join(t.TempDir(), "no-such-dir", "out.log")
	_, err := pm.Spawn("echo hi", "", SpawnOptions{LogFile: badPath})
	if err == nil {
		t.Fatal("expected an error when the log file's parent directory does not exist")
	}
	if len(pm.List()) != 0 {
		t.Errorf("expected no process to be registered after a failed spawn, got %d", len(pm.List()))
	}
}

func TestProcessManager_CleanAll_StopsAndRemovesRunningProcesses(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	doneID, err := pm.Spawn("echo already-done", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	waitForState(t, pm, doneID, domain.StateDone, 5*time.Second)

	runningID, err := pm.Spawn("sleep 300", "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	waitForState(t, pm, runningID, domain.StateRunning, 5*time.Second)

	count := pm.CleanAll(false)
	if count != 2 {
		t.Fatalf("expected 2 processes cleaned, got %d", count)
	}
	if remaining := pm.List(); len(remaining) != 0 {
		t.Errorf("expected all processes removed after CleanAll, got %d remaining: %+v", len(remaining), remaining)
	}
}

func TestProcessManager_CleanAll_NonForceLeavesSigtermIgnoringProcessRunning(t *testing.T) {
	// Edge case: a process that traps SIGTERM should NOT be silently dropped
	// from tracking just because CleanAll asked it to stop — CleanAll must
	// only remove processes that actually exited.
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn(`sh -c 'trap "" TERM; echo trap-ready; sleep 300'`, "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	// Wait for proof the trap is actually installed rather than guessing a
	// fixed delay — on a slow/contended CI runner, "sh" may not have reached
	// the trap statement yet, and signaling too early would kill it for
	// free, making this edge case impossible to observe.
	waitForOutput(t, pm, id, "trap-ready", 5*time.Second)

	count := pm.CleanAll(false)
	if count != 0 {
		t.Errorf("expected 0 processes cleaned (SIGTERM is ignored), got %d", count)
	}

	list := pm.List()
	if len(list) != 1 || list[0].ID != id || list[0].State != domain.StateRunning {
		t.Fatalf("expected process %d to remain tracked and running, got: %+v", id, list)
	}
}

func TestProcessManager_CleanAll_ForceKillsSigtermIgnoringProcess(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn(`sh -c 'trap "" TERM; echo trap-ready; sleep 300'`, "", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	waitForOutput(t, pm, id, "trap-ready", 5*time.Second)

	count := pm.CleanAll(true)
	if count != 1 {
		t.Fatalf("expected 1 process force-cleaned, got %d", count)
	}
	if remaining := pm.List(); len(remaining) != 0 {
		t.Errorf("expected process removed after force CleanAll, got %d remaining", len(remaining))
	}
}
