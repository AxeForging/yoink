package services

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/AxeForging/yoink/domain"
)

type managedProcess struct {
	info   domain.ProcessInfo
	pty    io.ReadWriteCloser
	cmd    *exec.Cmd
	buffer *RingBuffer

	attachMu  sync.Mutex
	attachOut io.Writer
}

// ProcessManager manages background processes with PTY I/O.
type ProcessManager struct {
	processes map[int]*managedProcess
	nextID    int
	mu        sync.RWMutex
	bufSize   int
}

// NewProcessManager creates a process manager with the given ring buffer size per process.
func NewProcessManager(bufSize int) *ProcessManager {
	return &ProcessManager{
		processes: make(map[int]*managedProcess),
		nextID:    1,
		bufSize:   bufSize,
	}
}

// Spawn starts a new background process and returns its ID.
func (pm *ProcessManager) Spawn(command string) (int, error) {
	pm.mu.Lock()
	id := pm.nextID
	pm.nextID++
	pm.mu.Unlock()

	cmd := exec.Command("sh", "-c", command)
	ptmx, err := StartProcessWithPTY(cmd)
	if err != nil {
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	buf := NewRingBuffer(pm.bufSize)
	now := time.Now()

	mp := &managedProcess{
		info: domain.ProcessInfo{
			ID:        id,
			Command:   command,
			State:     domain.StateRunning,
			PID:       cmd.Process.Pid,
			StartedAt: now,
		},
		pty:    ptmx,
		cmd:    cmd,
		buffer: buf,
	}

	pm.mu.Lock()
	pm.processes[id] = mp
	pm.mu.Unlock()

	go pm.captureOutput(mp)
	go pm.monitorProcess(mp)

	return id, nil
}

func (pm *ProcessManager) captureOutput(mp *managedProcess) {
	buf := make([]byte, 4096)
	var partial string
	for {
		n, err := mp.pty.Read(buf)
		if n > 0 {
			data := buf[:n]

			mp.attachMu.Lock()
			if mp.attachOut != nil {
				mp.attachOut.Write(data) //nolint:errcheck
			}
			mp.attachMu.Unlock()

			partial += string(data)
			for {
				idx := strings.Index(partial, "\n")
				if idx < 0 {
					break
				}
				line := strings.TrimRight(partial[:idx], "\r")
				partial = partial[idx+1:]
				mp.buffer.Write(line)
			}
		}
		if err != nil {
			if partial != "" {
				mp.buffer.Write(strings.TrimRight(partial, "\r"))
			}
			break
		}
	}
}

func (pm *ProcessManager) monitorProcess(mp *managedProcess) {
	err := mp.cmd.Wait()
	now := time.Now()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	mp.info.FinishedAt = &now
	if err != nil {
		mp.info.State = domain.StateFailed
		if exitErr, ok := err.(*exec.ExitError); ok {
			mp.info.ExitCode = exitErr.ExitCode()
		} else {
			mp.info.ExitCode = -1
		}
	} else {
		mp.info.State = domain.StateDone
		mp.info.ExitCode = 0
	}
}

// List returns info for all managed processes.
func (pm *ProcessManager) List() []domain.ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	list := make([]domain.ProcessInfo, 0, len(pm.processes))
	for _, mp := range pm.processes {
		list = append(list, mp.info)
	}
	return list
}

// GetOutput returns the last n lines of output for the given process.
func (pm *ProcessManager) GetOutput(id, lines int) (string, error) {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("process %d not found", id)
	}
	result := mp.buffer.ReadLast(lines)
	return strings.Join(result, "\n"), nil
}

// SendInput writes text followed by a newline to the process PTY.
func (pm *ProcessManager) SendInput(id int, input string) error {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process %d not found", id)
	}
	if mp.info.State != domain.StateRunning {
		return fmt.Errorf("process %d is not running", id)
	}
	_, err := mp.pty.Write([]byte(input + "\n"))
	return err
}

// Kill sends a signal to the process. Use force=true for SIGKILL.
func (pm *ProcessManager) Kill(id int, force bool) error {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process %d not found", id)
	}
	if mp.info.State != domain.StateRunning {
		return fmt.Errorf("process %d is not running", id)
	}
	if force {
		return KillProcess(mp.cmd.Process)
	}
	return TerminateProcess(mp.cmd.Process)
}

// GetPTY returns the PTY file descriptor for attach operations.
func (pm *ProcessManager) GetPTY(id int) (io.ReadWriteCloser, error) {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("process %d not found", id)
	}
	if mp.info.State != domain.StateRunning {
		return nil, fmt.Errorf("process %d is not running", id)
	}
	return mp.pty, nil
}

// AttachOutput registers a writer to receive live output for the given process.
func (pm *ProcessManager) AttachOutput(id int, w io.Writer) error {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process %d not found", id)
	}
	mp.attachMu.Lock()
	mp.attachOut = w
	mp.attachMu.Unlock()
	return nil
}

// DetachOutput removes the live output writer for the given process.
func (pm *ProcessManager) DetachOutput(id int) {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if mp == nil || !ok {
		return
	}
	mp.attachMu.Lock()
	mp.attachOut = nil
	mp.attachMu.Unlock()
}

// Clean removes all finished/failed processes and returns the count removed.
func (pm *ProcessManager) Clean() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	count := 0
	for id, mp := range pm.processes {
		if mp.info.State != domain.StateRunning {
			mp.pty.Close()
			delete(pm.processes, id)
			count++
		}
	}
	return count
}

// Shutdown terminates all running processes and cleans up.
func (pm *ProcessManager) Shutdown() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, mp := range pm.processes {
		if mp.info.State == domain.StateRunning {
			TerminateProcess(mp.cmd.Process) //nolint:errcheck
		}
		mp.pty.Close()
	}
}
