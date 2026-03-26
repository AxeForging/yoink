//go:build !windows

package services

import (
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

	id, err := pm.Spawn("echo hello-yoink", "")
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
	id, err := pm.Spawn("cat", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond) // let cat start

	if err := pm.SendInput(id, "test-input-123", false); err != nil {
		t.Fatalf("SendInput failed: %v", err)
	}

	waitForOutput(t, pm, id, "test-input-123", 5*time.Second)
}

func TestProcessManager_SpawnFailedExitCode(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("exit 42", "")
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

	id, err := pm.Spawn("sleep 300", "")
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

	id, err := pm.Spawn("sleep 300", "")
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

	pm.Spawn("echo one", "")
	pm.Spawn("echo two", "")
	pm.Spawn("sleep 300", "")

	time.Sleep(500 * time.Millisecond)

	list := pm.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(list))
	}
}

func TestProcessManager_Clean(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	pm.Spawn("echo done-quick", "")
	id2, _ := pm.Spawn("sleep 300", "")

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

	id, _ := pm.Spawn("echo fast", "")
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	err := pm.SendInput(id, "too late", false)
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

	id, err := pm.Spawn("printf 'line1\\nline2\\nline3\\n'", "")
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

	id, err := pm.Spawn("cat", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	pm.SendInput(id, "hello", false)
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

	id, err := pm.Spawn("cat", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	pm.SendInput(id, "super-secret-token", true)
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

func TestProcessManager_SpawnWithAlias(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("echo aliased", "deploy")
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

	id, err := pm.Spawn("sleep 300", "myproc")
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

	pm.Spawn("sleep 300", "")

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

	_, err := pm.Spawn("sleep 300", "dup")
	if err != nil {
		t.Fatalf("first Spawn failed: %v", err)
	}

	_, err = pm.Spawn("sleep 300", "dup")
	if err == nil {
		t.Error("expected error for duplicate alias")
	}
}

func TestProcessManager_AliasReuseAfterClean(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, _ := pm.Spawn("echo fast", "reuse-me")
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	pm.Clean()

	// Alias should be available again
	id2, err := pm.Spawn("echo again", "reuse-me")
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

	id, _ := pm.Spawn("cat", "interactive")
	time.Sleep(200 * time.Millisecond)

	// Resolve alias and use for operations
	resolved, _ := pm.ResolveID("interactive")
	if err := pm.SendInput(resolved, "hello-alias", false); err != nil {
		t.Fatalf("SendInput via alias failed: %v", err)
	}

	waitForOutput(t, pm, id, "hello-alias", 5*time.Second)

	// Kill via resolved alias
	if err := pm.Kill(resolved, false); err != nil {
		t.Fatalf("Kill via alias failed: %v", err)
	}
	waitForState(t, pm, id, domain.StateFailed, 5*time.Second)
}
