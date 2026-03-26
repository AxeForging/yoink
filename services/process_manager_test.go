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
		output, _ := pm.GetOutput(id, 1000)
		if strings.Contains(output, expected) {
			return
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

	id, err := pm.Spawn("echo hello-yoink")
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
	id, err := pm.Spawn("cat")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond) // let cat start

	if err := pm.SendInput(id, "test-input-123"); err != nil {
		t.Fatalf("SendInput failed: %v", err)
	}

	waitForOutput(t, pm, id, "test-input-123", 5*time.Second)
}

func TestProcessManager_SpawnFailedExitCode(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	id, err := pm.Spawn("exit 42")
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

	id, err := pm.Spawn("sleep 300")
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

	id, err := pm.Spawn("sleep 300")
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

	pm.Spawn("echo one")
	pm.Spawn("echo two")
	pm.Spawn("sleep 300")

	time.Sleep(500 * time.Millisecond)

	list := pm.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(list))
	}
}

func TestProcessManager_Clean(t *testing.T) {
	pm := NewProcessManager(100)
	defer pm.Shutdown()

	pm.Spawn("echo done-quick")
	id2, _ := pm.Spawn("sleep 300")

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

	id, _ := pm.Spawn("echo fast")
	waitForState(t, pm, id, domain.StateDone, 5*time.Second)

	err := pm.SendInput(id, "too late")
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

	id, err := pm.Spawn("printf 'line1\\nline2\\nline3\\n'")
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
