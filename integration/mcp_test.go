//go:build !windows

package integration

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AxeForging/yoink/domain"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// startMCP launches the real binary as `yoink mcp` and speaks MCP to it over
// stdio, exactly as an agent's MCP client would. No daemon is started up front:
// the server is expected to auto-start one, so an agent needs no setup step.
func startMCP(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	bin := buildBinary(t)
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "yoink.sock")

	cmd := exec.Command(bin, "mcp")
	cmd.Env = append(os.Environ(), "YOINK_SOCK="+sockPath, "HOME="+sockDir)
	cmd.Stderr = os.Stderr

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "yoink-integration", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcpsdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		cancel()
		t.Fatalf("connect to yoink mcp over stdio: %v", err)
	}

	t.Cleanup(func() {
		cs.Close() //nolint:errcheck
		cancel()
		// Shut down the daemon the MCP server auto-started.
		if conn, derr := net.Dial("unix", sockPath); derr == nil {
			json.NewEncoder(conn).Encode(domain.DaemonRequest{Type: domain.ReqShutdown}) //nolint:errcheck
			conn.Close()
		}
	})
	return cs
}

func callTool(t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if res.IsError {
		var sb strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcpsdk.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		t.Fatalf("%s returned error: %s", name, sb.String())
	}
	if out == nil {
		return
	}
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("decode %s result: %v", name, err)
		}
		return
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), out); err != nil {
				t.Fatalf("decode %s text result: %v", name, err)
			}
			return
		}
	}
	t.Fatalf("%s returned no decodable content", name)
}

func TestIntegration_MCPStdioHandshakeAndToolList(t *testing.T) {
	cs := startMCP(t)

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(res.Tools) != 8 {
		names := make([]string, len(res.Tools))
		for i, tool := range res.Tools {
			names[i] = tool.Name
		}
		t.Fatalf("expected 8 tools over stdio, got %d: %v", len(res.Tools), names)
	}
}

func TestIntegration_MCPRunsAndWaitsThroughTheShippedBinary(t *testing.T) {
	cs := startMCP(t)
	dir := t.TempDir()

	var run struct {
		ID  int    `json:"id"`
		Cwd string `json:"cwd"`
	}
	callTool(t, cs, "yoink_run", map[string]any{
		"command": "sh -c 'echo stdio-round-trip; exit 0'",
		"alias":   "roundtrip",
		"cwd":     dir,
	}, &run)
	if run.ID < 1 {
		t.Fatalf("expected a process id, got %d", run.ID)
	}

	var wait struct {
		Done      bool `json:"done"`
		TimedOut  bool `json:"timedOut"`
		Failed    bool `json:"failed"`
		Processes []struct {
			State    string `json:"state"`
			ExitCode *int   `json:"exitCode"`
			Output   string `json:"output"`
		} `json:"processes"`
	}
	callTool(t, cs, "yoink_wait", map[string]any{
		"processes": []any{"roundtrip"}, "timeoutSeconds": 20, "pollSeconds": 1,
	}, &wait)

	if !wait.Done || wait.TimedOut || wait.Failed {
		t.Fatalf("expected a clean finish, got done=%v timedOut=%v failed=%v", wait.Done, wait.TimedOut, wait.Failed)
	}
	if len(wait.Processes) != 1 {
		t.Fatalf("expected one process, got %d", len(wait.Processes))
	}
	p := wait.Processes[0]
	if p.State != "done" {
		t.Errorf("expected state done, got %s", p.State)
	}
	if p.ExitCode == nil || *p.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %v", p.ExitCode)
	}
	if !strings.Contains(p.Output, "stdio-round-trip") {
		t.Errorf("expected the process output in the wait result, got %q", p.Output)
	}

	var list struct {
		Processes []domain.ProcessInfo `json:"processes"`
	}
	callTool(t, cs, "yoink_ls", map[string]any{}, &list)
	if len(list.Processes) != 1 || list.Processes[0].Alias != "roundtrip" {
		t.Fatalf("expected the process in the list, got %+v", list.Processes)
	}

	var clean struct {
		Cleaned int `json:"cleaned"`
	}
	callTool(t, cs, "yoink_clean", map[string]any{"all": true}, &clean)
	if clean.Cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", clean.Cleaned)
	}
}

// The MCP server shares the daemon with the CLI, so a process started by one is
// visible to the other. This is the property that lets an agent inspect a dev
// server a human started in their own terminal.
func TestIntegration_MCPAndCLIShareTheSameDaemon(t *testing.T) {
	bin := buildBinary(t)
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "yoink.sock")
	env := append(os.Environ(), "YOINK_SOCK="+sockPath, "HOME="+sockDir)

	// Start a process the way a human would, from the CLI.
	cliRun := exec.Command(bin, "run", "--alias", "human-side", "sh -c 'sleep 30'")
	cliRun.Env = env
	if out, err := cliRun.CombinedOutput(); err != nil {
		t.Fatalf("cli run failed: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	mcpCmd := exec.Command(bin, "mcp")
	mcpCmd.Env = env
	mcpCmd.Stderr = os.Stderr
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "yoink-integration", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcpsdk.CommandTransport{Command: mcpCmd}, nil)
	if err != nil {
		t.Fatalf("connect to yoink mcp: %v", err)
	}
	defer func() {
		cs.Close() //nolint:errcheck
		if conn, derr := net.Dial("unix", sockPath); derr == nil {
			json.NewEncoder(conn).Encode(domain.DaemonRequest{Type: domain.ReqShutdown}) //nolint:errcheck
			conn.Close()
		}
	}()

	var list struct {
		Processes []domain.ProcessInfo `json:"processes"`
	}
	callTool(t, cs, "yoink_ls", map[string]any{}, &list)

	var found bool
	for _, p := range list.Processes {
		if p.Alias == "human-side" && p.State == domain.StateRunning {
			found = true
		}
	}
	if !found {
		t.Fatalf("MCP did not see the CLI-started process; got %+v", list.Processes)
	}

	// And the reverse: killing it over MCP must be visible to the CLI.
	callTool(t, cs, "yoink_kill", map[string]any{"process": "human-side", "force": true}, nil)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		cliLs := exec.Command(bin, "ls")
		cliLs.Env = env
		out, err := cliLs.CombinedOutput()
		if err != nil {
			t.Fatalf("cli ls failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "running") {
			return // CLI observes the MCP-issued kill
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("CLI still reports the process as running after an MCP kill")
}

// stdout is the MCP transport. Anything else written there corrupts the
// protocol stream, so a successful handshake plus a clean tool round-trip is
// the regression test for it.
func TestIntegration_MCPKeepsStdoutClean(t *testing.T) {
	cs := startMCP(t)

	var out struct {
		Output string `json:"output"`
	}
	callTool(t, cs, "yoink_run", map[string]any{
		"command": "echo clean-stdout", "alias": "clean",
	}, nil)
	callTool(t, cs, "yoink_wait", map[string]any{
		"processes": []any{"clean"}, "timeoutSeconds": 20, "pollSeconds": 1,
	}, nil)
	callTool(t, cs, "yoink_log", map[string]any{"process": "clean"}, &out)

	if !strings.Contains(out.Output, "clean-stdout") {
		t.Errorf("expected process output, got %q", out.Output)
	}
}
