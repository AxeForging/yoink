//go:build !windows

package services

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AxeForging/yoink/domain"
	"github.com/rs/zerolog"
)

func startTestDaemon(t *testing.T) (*DaemonServer, string) {
	t.Helper()
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	pm := NewProcessManager(100)
	log := zerolog.New(zerolog.NewTestWriter(t))
	server := NewDaemonServer(sockPath, pm, log)

	go server.Start()

	// Wait for socket to be ready
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			// Socket file exists, try a quick list request to verify
			conn, err := net.Dial("unix", sockPath)
			if err == nil {
				json.NewEncoder(conn).Encode(domain.DaemonRequest{Type: domain.ReqList})
				var resp domain.DaemonResponse
				json.NewDecoder(conn).Decode(&resp)
				conn.Close()
				if resp.Success {
					break
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Cleanup(func() {
		server.Stop()
	})

	return server, sockPath
}

func sendRequest(t *testing.T, sockPath string, req domain.DaemonRequest) domain.DaemonResponse {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("failed to send request: %v", err)
	}

	var resp domain.DaemonResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	return resp
}

func TestDaemon_RunAndList(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	// Run a command
	resp := sendRequest(t, sockPath, domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo daemon-test",
	})
	if !resp.Success {
		t.Fatalf("run failed: %s", resp.Error)
	}
	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %d", resp.ID)
	}

	time.Sleep(500 * time.Millisecond)

	// List processes
	listResp := sendRequest(t, sockPath, domain.DaemonRequest{Type: domain.ReqList})
	if !listResp.Success {
		t.Fatalf("list failed: %s", listResp.Error)
	}
	if len(listResp.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(listResp.Processes))
	}
	if listResp.Processes[0].Command != "echo daemon-test" {
		t.Errorf("unexpected command: %s", listResp.Processes[0].Command)
	}
}

func TestDaemon_LogOutput(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	sendRequest(t, sockPath, domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo hello-from-daemon",
	})

	time.Sleep(500 * time.Millisecond)

	resp := sendRequest(t, sockPath, domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    1,
		Lines: 10,
	})
	if !resp.Success {
		t.Fatalf("log failed: %s", resp.Error)
	}
	if !strings.Contains(resp.Output, "hello-from-daemon") {
		t.Errorf("expected output to contain 'hello-from-daemon', got: %q", resp.Output)
	}
}

func TestDaemon_SendInput(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	sendRequest(t, sockPath, domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "cat",
	})

	time.Sleep(300 * time.Millisecond)

	resp := sendRequest(t, sockPath, domain.DaemonRequest{
		Type:  domain.ReqSend,
		ID:    1,
		Input: "daemon-input-test",
	})
	if !resp.Success {
		t.Fatalf("send failed: %s", resp.Error)
	}

	time.Sleep(500 * time.Millisecond)

	logResp := sendRequest(t, sockPath, domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    1,
		Lines: 10,
	})
	if !logResp.Success {
		t.Fatalf("log failed: %s", logResp.Error)
	}
	if !strings.Contains(logResp.Output, "daemon-input-test") {
		t.Errorf("expected 'daemon-input-test' in output, got: %q", logResp.Output)
	}
}

func TestDaemon_Kill(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	sendRequest(t, sockPath, domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sleep 300",
	})

	time.Sleep(300 * time.Millisecond)

	resp := sendRequest(t, sockPath, domain.DaemonRequest{
		Type: domain.ReqKill,
		ID:   1,
	})
	if !resp.Success {
		t.Fatalf("kill failed: %s", resp.Error)
	}

	time.Sleep(500 * time.Millisecond)

	listResp := sendRequest(t, sockPath, domain.DaemonRequest{Type: domain.ReqList})
	if !listResp.Success {
		t.Fatalf("list failed: %s", listResp.Error)
	}
	if len(listResp.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(listResp.Processes))
	}
	if listResp.Processes[0].State != domain.StateFailed {
		t.Errorf("expected state 'failed', got %s", listResp.Processes[0].State)
	}
}

func TestDaemon_Clean(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	// Run a quick command that finishes
	sendRequest(t, sockPath, domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "echo clean-me",
	})

	// Run a long command
	sendRequest(t, sockPath, domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sleep 300",
	})

	time.Sleep(500 * time.Millisecond)

	cleanResp := sendRequest(t, sockPath, domain.DaemonRequest{Type: domain.ReqClean})
	if !cleanResp.Success {
		t.Fatalf("clean failed: %s", cleanResp.Error)
	}
	if cleanResp.Cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", cleanResp.Cleaned)
	}

	listResp := sendRequest(t, sockPath, domain.DaemonRequest{Type: domain.ReqList})
	if len(listResp.Processes) != 1 {
		t.Errorf("expected 1 remaining process, got %d", len(listResp.Processes))
	}
}

func TestDaemon_LogNotFound(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	resp := sendRequest(t, sockPath, domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    999,
		Lines: 10,
	})
	if resp.Success {
		t.Error("expected failure for non-existent process")
	}
	if resp.Error == "" {
		t.Error("expected error message")
	}
}

func TestDaemon_MultipleProcesses(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	for i := 0; i < 5; i++ {
		resp := sendRequest(t, sockPath, domain.DaemonRequest{
			Type:    domain.ReqRun,
			Command: "echo proc-" + os.Getenv("USER"),
		})
		if !resp.Success {
			t.Fatalf("run %d failed: %s", i, resp.Error)
		}
		if resp.ID != i+1 {
			t.Errorf("expected ID %d, got %d", i+1, resp.ID)
		}
	}

	time.Sleep(500 * time.Millisecond)

	listResp := sendRequest(t, sockPath, domain.DaemonRequest{Type: domain.ReqList})
	if len(listResp.Processes) != 5 {
		t.Errorf("expected 5 processes, got %d", len(listResp.Processes))
	}
}

func TestDaemon_Shutdown(t *testing.T) {
	_, sockPath := startTestDaemon(t)

	// Run something
	sendRequest(t, sockPath, domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: "sleep 300",
	})

	time.Sleep(200 * time.Millisecond)

	// Shutdown
	resp := sendRequest(t, sockPath, domain.DaemonRequest{Type: domain.ReqShutdown})
	if !resp.Success {
		t.Fatalf("shutdown failed: %s", resp.Error)
	}

	// Socket should be gone
	time.Sleep(500 * time.Millisecond)
	_, err := net.Dial("unix", sockPath)
	if err == nil {
		t.Error("expected connection to fail after shutdown")
	}
}
