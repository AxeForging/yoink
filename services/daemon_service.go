package services

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/AxeForging/yoink/domain"
	"github.com/rs/zerolog"
)

// DaemonServer listens on a Unix socket and manages background processes.
type DaemonServer struct {
	pm       *ProcessManager
	sockPath string
	listener net.Listener
	log      zerolog.Logger
	wg       sync.WaitGroup
	done     chan struct{}
}

// NewDaemonServer creates a new daemon server.
func NewDaemonServer(sockPath string, pm *ProcessManager, log zerolog.Logger) *DaemonServer {
	return &DaemonServer{
		pm:       pm,
		sockPath: sockPath,
		log:      log,
		done:     make(chan struct{}),
	}
}

// Start begins listening for client connections. Blocks until Stop is called.
func (ds *DaemonServer) Start() error {
	// Clear a socket left behind by a previous run; absence is the normal case.
	_ = os.Remove(ds.sockPath)

	listener, err := net.Listen("unix", ds.sockPath)
	if err != nil {
		return err
	}
	ds.listener = listener
	ds.log.Info().Str("socket", ds.sockPath).Msg("daemon started")

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			ds.log.Error().Err(err).Msg("accept error")
			continue
		}
		ds.wg.Add(1)
		go func() {
			defer ds.wg.Done()
			ds.handleConnection(conn)
		}()
	}
}

// Stop closes the listener and removes the socket file.
func (ds *DaemonServer) Stop() {
	if ds.listener != nil {
		_ = ds.listener.Close()
	}
	ds.wg.Wait()
	_ = os.Remove(ds.sockPath)
}

func (ds *DaemonServer) handleConnection(conn net.Conn) {
	var req domain.DaemonRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		ds.log.Error().Err(err).Msg("decode error")
		conn.Close() //nolint:errcheck
		return
	}

	// Resolve alias to ID for commands that reference a process
	if req.Alias != "" && req.ID == 0 && req.Type != domain.ReqRun {
		id, err := ds.pm.ResolveID(req.Alias)
		if err != nil {
			ds.sendResponse(conn, domain.DaemonResponse{Error: err.Error()})
			conn.Close() //nolint:errcheck
			return
		}
		req.ID = id
	}

	ds.log.Debug().Str("type", string(req.Type)).Int("id", req.ID).Msg("request")

	switch req.Type {
	case domain.ReqRun:
		ds.handleRun(conn, req)
	case domain.ReqList:
		ds.handleList(conn)
	case domain.ReqLog:
		ds.handleLog(conn, req)
	case domain.ReqSend:
		ds.handleSend(conn, req)
	case domain.ReqAttach:
		ds.handleAttach(conn, req)
		return
	case domain.ReqSnapshot:
		ds.handleSnapshot(conn, req)
	case domain.ReqKill:
		ds.handleKill(conn, req)
	case domain.ReqClean:
		ds.handleClean(conn, req)
	case domain.ReqShutdown:
		ds.handleShutdown(conn)
		return
	default:
		ds.sendResponse(conn, domain.DaemonResponse{Error: "unknown request type"})
		conn.Close() //nolint:errcheck
	}
}

func (ds *DaemonServer) sendResponse(conn net.Conn, resp domain.DaemonResponse) {
	json.NewEncoder(conn).Encode(resp) //nolint:errcheck
}

func (ds *DaemonServer) handleRun(conn net.Conn, req domain.DaemonRequest) {
	defer conn.Close() //nolint:errcheck
	id, err := ds.pm.Spawn(req.Command, req.Alias, SpawnOptions{Cwd: req.Cwd, LogFile: req.LogFile})
	if err != nil {
		ds.sendResponse(conn, domain.DaemonResponse{Error: err.Error()})
		return
	}
	ds.log.Info().Int("id", id).Str("alias", req.Alias).Str("cmd", req.Command).Msg("spawned process")
	ds.sendResponse(conn, domain.DaemonResponse{Success: true, ID: id})
}

func (ds *DaemonServer) handleList(conn net.Conn) {
	defer conn.Close() //nolint:errcheck
	processes := ds.pm.List()
	ds.sendResponse(conn, domain.DaemonResponse{Success: true, Processes: processes})
}

func (ds *DaemonServer) handleLog(conn net.Conn, req domain.DaemonRequest) {
	defer conn.Close() //nolint:errcheck
	var output string
	var err error
	if req.New {
		output, err = ds.pm.GetNewOutput(req.ID)
	} else {
		lines := req.Lines
		if lines <= 0 {
			lines = 50
		}
		output, err = ds.pm.GetOutput(req.ID, lines)
	}
	if err != nil {
		ds.sendResponse(conn, domain.DaemonResponse{Error: err.Error()})
		return
	}
	ds.sendResponse(conn, domain.DaemonResponse{Success: true, ID: req.ID, Output: output})
}

func (ds *DaemonServer) handleSend(conn net.Conn, req domain.DaemonRequest) {
	defer conn.Close() //nolint:errcheck
	opts := SendOptions{
		Redacted:  req.Redacted,
		TypeMode:  req.TypeMode,
		TypeDelay: time.Duration(req.TypeDelay) * time.Millisecond,
		NoEnter:   req.NoEnter,
		Submit:    req.Submit,
	}
	if err := ds.pm.SendInput(req.ID, req.Input, opts); err != nil {
		ds.sendResponse(conn, domain.DaemonResponse{Error: err.Error()})
		return
	}
	ds.sendResponse(conn, domain.DaemonResponse{Success: true, ID: req.ID})
}

func (ds *DaemonServer) handleAttach(conn net.Conn, req domain.DaemonRequest) {
	ptmx, err := ds.pm.GetPTY(req.ID)
	if err != nil {
		ds.sendResponse(conn, domain.DaemonResponse{Error: err.Error()})
		conn.Close() //nolint:errcheck
		return
	}

	ds.sendResponse(conn, domain.DaemonResponse{Success: true, ID: req.ID})

	ds.pm.AttachOutput(req.ID, conn) //nolint:errcheck
	defer ds.pm.DetachOutput(req.ID)

	// Forward input: conn -> pty
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			ptmx.Write(buf[:n]) //nolint:errcheck
		}
		if err != nil {
			break
		}
	}
	conn.Close() //nolint:errcheck
}

func (ds *DaemonServer) handleSnapshot(conn net.Conn, req domain.DaemonRequest) {
	defer conn.Close() //nolint:errcheck
	output, err := ds.pm.Snapshot(req.ID)
	if err != nil {
		ds.sendResponse(conn, domain.DaemonResponse{Error: err.Error()})
		return
	}
	ds.sendResponse(conn, domain.DaemonResponse{Success: true, ID: req.ID, Output: output})
}

func (ds *DaemonServer) handleKill(conn net.Conn, req domain.DaemonRequest) {
	defer conn.Close() //nolint:errcheck
	if err := ds.pm.Kill(req.ID, req.Force); err != nil {
		ds.sendResponse(conn, domain.DaemonResponse{Error: err.Error()})
		return
	}
	ds.sendResponse(conn, domain.DaemonResponse{Success: true, ID: req.ID})
}

func (ds *DaemonServer) handleClean(conn net.Conn, req domain.DaemonRequest) {
	defer conn.Close() //nolint:errcheck
	var count int
	if req.All {
		count = ds.pm.CleanAll(req.Force)
	} else {
		count = ds.pm.Clean()
	}
	ds.sendResponse(conn, domain.DaemonResponse{Success: true, Cleaned: count})
}

func (ds *DaemonServer) handleShutdown(conn net.Conn) {
	ds.sendResponse(conn, domain.DaemonResponse{Success: true})
	conn.Close() //nolint:errcheck
	ds.pm.Shutdown()
	// Signal the main loop to stop without waiting for this goroutine
	if ds.listener != nil {
		_ = ds.listener.Close()
	}
	_ = os.Remove(ds.sockPath)
	close(ds.done)
}

// HandleSingleRequest processes one request on the given connection.
// Exported for testing.
func (ds *DaemonServer) HandleSingleRequest(conn io.ReadWriteCloser) {
	netConn, ok := conn.(net.Conn)
	if !ok {
		return
	}
	ds.handleConnection(netConn)
}
