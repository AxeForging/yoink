package services

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/AxeForging/yoink/domain"
)

// ClientService communicates with the yoink daemon over a Unix socket.
type ClientService struct {
	SockPath string
}

// NewClientService creates a client that connects to the given socket path.
func NewClientService(sockPath string) *ClientService {
	return &ClientService{SockPath: sockPath}
}

// Connect establishes a connection to the daemon socket.
func (cs *ClientService) Connect() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", cs.SockPath, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to daemon: %w", err)
	}
	return conn, nil
}

// EnsureDaemon checks if the daemon is running and starts it if not.
func (cs *ClientService) EnsureDaemon() error {
	conn, err := net.DialTimeout("unix", cs.SockPath, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		return nil
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find yoink binary: %w", err)
	}

	logDir := filepath.Dir(cs.SockPath)
	os.MkdirAll(logDir, 0o755) //nolint:errcheck
	logPath := filepath.Join(logDir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("cannot create daemon log: %w", err)
	}

	cmd := exec.Command(binPath, "daemon")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = DaemonSysProcAttr()

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}
	logFile.Close()

	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		conn, err := net.DialTimeout("unix", cs.SockPath, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
	}
	return fmt.Errorf("daemon failed to start within 3 seconds")
}

// SendRequest encodes and sends a request to the daemon.
func (cs *ClientService) SendRequest(conn net.Conn, req domain.DaemonRequest) error {
	return json.NewEncoder(conn).Encode(req)
}

// ReadResponse reads and decodes a response from the daemon.
func (cs *ClientService) ReadResponse(conn net.Conn) (domain.DaemonResponse, error) {
	var resp domain.DaemonResponse
	err := json.NewDecoder(conn).Decode(&resp)
	return resp, err
}

// Do sends a request and returns the response, auto-starting the daemon if needed.
func (cs *ClientService) Do(req domain.DaemonRequest) (domain.DaemonResponse, error) {
	if err := cs.EnsureDaemon(); err != nil {
		return domain.DaemonResponse{}, err
	}

	conn, err := cs.Connect()
	if err != nil {
		return domain.DaemonResponse{}, err
	}
	defer conn.Close()

	if err := cs.SendRequest(conn, req); err != nil {
		return domain.DaemonResponse{}, err
	}

	return cs.ReadResponse(conn)
}
