package integration

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AxeForging/yoink/domain"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// shortSockPath returns a socket path that fits macOS's 104-byte sun_path
// limit; t.TempDir() under /var/folders plus a long test name overflows it.
func shortSockPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "yk")
	if err != nil {
		t.Fatalf("create socket dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck
	return filepath.Join(dir, "yoink.sock")
}

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
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
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
	sockPath := shortSockPath(t)
	sockDir := filepath.Dir(sockPath)

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
			conn.Close() //nolint:errcheck
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Cleanup(func() {
		// Shutdown daemon via socket
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			json.NewEncoder(conn).Encode(domain.DaemonRequest{Type: domain.ReqShutdown}) //nolint:errcheck
			conn.Close()                                                                 //nolint:errcheck
		}
		daemonCmd.Process.Kill() //nolint:errcheck
		daemonCmd.Wait()         //nolint:errcheck
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

// startMCP launches the real binary as `yoink mcp` and speaks MCP to it over
// stdio, exactly as an agent's MCP client would. No daemon is started up front:
// the server is expected to auto-start one, so an agent needs no setup step.
func startMCP(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	bin := buildBinary(t)
	sockPath := shortSockPath(t)
	sockDir := filepath.Dir(sockPath)

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
			conn.Close()                                                                 //nolint:errcheck
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
