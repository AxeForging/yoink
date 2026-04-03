package services

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AxeForging/yoink/domain"
	"github.com/hinshun/vt10x"
)

type managedProcess struct {
	info   domain.ProcessInfo
	pty    io.ReadWriteCloser
	cmd    *exec.Cmd
	buffer *RingBuffer
	vt     vt10x.Terminal // VT100 emulator for snapshot support

	attachMu  sync.Mutex
	attachOut io.Writer

	redactMu       sync.Mutex
	redactPatterns []string // patterns to redact from stdout (from send-redacted echo)

	logCursor int // tracks position for --new log reads
}

// ProcessManager manages background processes with PTY I/O.
type ProcessManager struct {
	processes map[int]*managedProcess
	aliases   map[string]int
	nextID    int
	mu        sync.RWMutex
	bufSize   int
}

// NewProcessManager creates a process manager with the given ring buffer size per process.
func NewProcessManager(bufSize int) *ProcessManager {
	return &ProcessManager{
		processes: make(map[int]*managedProcess),
		aliases:   make(map[string]int),
		nextID:    1,
		bufSize:   bufSize,
	}
}

// Spawn starts a new background process and returns its ID.
func (pm *ProcessManager) Spawn(command, alias string) (int, error) {
	pm.mu.Lock()
	if alias != "" {
		if _, exists := pm.aliases[alias]; exists {
			pm.mu.Unlock()
			return 0, fmt.Errorf("alias %q is already in use", alias)
		}
	}
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
			Alias:     alias,
			Command:   command,
			State:     domain.StateRunning,
			PID:       cmd.Process.Pid,
			StartedAt: now,
		},
		pty:    ptmx,
		cmd:    cmd,
		buffer: buf,
		vt:     vt10x.New(vt10x.WithSize(220, 50)),
	}

	pm.mu.Lock()
	pm.processes[id] = mp
	if alias != "" {
		pm.aliases[alias] = id
	}
	pm.mu.Unlock()

	go pm.captureOutput(mp)
	go pm.monitorProcess(mp)

	return id, nil
}

// ResolveID resolves an alias or numeric ID string to a process ID.
func (pm *ProcessManager) ResolveID(ref string) (int, error) {
	// Try numeric ID first
	if id, err := strconv.Atoi(ref); err == nil {
		return id, nil
	}
	// Try alias lookup
	pm.mu.RLock()
	id, ok := pm.aliases[ref]
	pm.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("no process found with alias %q", ref)
	}
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

			mp.vt.Write(stripVTProblematic(data)) //nolint:errcheck

			partial += string(data)
			for {
				idx := strings.Index(partial, "\n")
				if idx < 0 {
					break
				}
				line := strings.TrimRight(partial[:idx], "\r")
				partial = partial[idx+1:]

				// Check if this line contains any redacted input (PTY echo or process output)
				redacted := false
				mp.redactMu.Lock()
				for _, pattern := range mp.redactPatterns {
					if strings.Contains(line, pattern) {
						redacted = true
						break
					}
				}
				mp.redactMu.Unlock()

				if redacted {
					mp.buffer.Write(domain.LogEntry{Source: domain.SourceStdout, Text: line, Redacted: true})
				} else {
					mp.buffer.Write(domain.LogEntry{Source: domain.SourceStdout, Text: line})
				}
			}
		}
		if err != nil {
			if partial != "" {
				mp.buffer.Write(domain.LogEntry{Source: domain.SourceStdout, Text: strings.TrimRight(partial, "\r")})
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

// GetOutput returns the last n lines of formatted output for the given process.
func (pm *ProcessManager) GetOutput(id, lines int) (string, error) {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("process %d not found", id)
	}
	entries := mp.buffer.ReadLast(lines)
	return FormatLogEntries(entries), nil
}

// GetNewOutput returns only the lines written since the last call and advances the cursor.
func (pm *ProcessManager) GetNewOutput(id int) (string, error) {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("process %d not found", id)
	}
	entries, newCursor := mp.buffer.ReadAfter(mp.logCursor)
	mp.logCursor = newCursor
	return FormatLogEntries(entries), nil
}

// GetLogEntries returns the last n raw log entries for the given process.
func (pm *ProcessManager) GetLogEntries(id, lines int) ([]domain.LogEntry, error) {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("process %d not found", id)
	}
	return mp.buffer.ReadLast(lines), nil
}

// SendOptions controls how input is delivered to a process PTY.
type SendOptions struct {
	Redacted  bool
	TypeMode  bool          // send character by character
	TypeDelay time.Duration // delay between characters in type mode
	NoEnter   bool          // omit the trailing newline
}

// SendInput writes text to the process PTY.
// By default it appends a newline; use SendOptions to customise behaviour.
func (pm *ProcessManager) SendInput(id int, input string, opts SendOptions) error {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process %d not found", id)
	}
	if mp.info.State != domain.StateRunning {
		return fmt.Errorf("process %d is not running", id)
	}

	// If redacted, register pattern to catch PTY echo
	if opts.Redacted {
		mp.redactMu.Lock()
		mp.redactPatterns = append(mp.redactPatterns, input)
		mp.redactMu.Unlock()
	}

	// Log the input in the ring buffer
	mp.buffer.Write(domain.LogEntry{
		Source:   domain.SourceStdin,
		Text:     input,
		Redacted: opts.Redacted,
	})

	if opts.TypeMode {
		for _, ch := range input {
			if _, err := mp.pty.Write([]byte(string(ch))); err != nil {
				return err
			}
			if opts.TypeDelay > 0 {
				time.Sleep(opts.TypeDelay)
			}
		}
	} else {
		if _, err := mp.pty.Write([]byte(input)); err != nil {
			return err
		}
	}

	if !opts.NoEnter {
		_, err := mp.pty.Write([]byte("\n"))
		return err
	}
	return nil
}

// FormatLogEntries formats log entries with source labels and redaction.
// ANSI/VT escape sequences are stripped so that log output never corrupts the
// caller's terminal (e.g. \x1b[?25l from a TUI process hiding the cursor).
func FormatLogEntries(entries []domain.LogEntry) string {
	if len(entries) == 0 {
		return ""
	}
	lines := make([]string, len(entries))
	for i, e := range entries {
		text := e.Text
		if e.Redacted {
			text = "********"
		}
		lines[i] = fmt.Sprintf("[%s] %s", e.Source, stripANSI(text))
	}
	return strings.Join(lines, "\n")
}

// stripANSI removes all ANSI/VT escape sequences from s so that the result is
// safe to print to any terminal without side-effects.  It handles:
//   - CSI sequences: ESC [ <params> <intermediates> <final>
//   - OSC sequences: ESC ] ... BEL  or  ESC ] ... ESC \
//   - All other two-byte ESC sequences (Fe / Fs / Fp)
func stripANSI(s string) string {
	b := []byte(s)
	out := make([]byte, 0, len(b))
	i := 0
	for i < len(b) {
		if b[i] != 0x1b || i+1 >= len(b) {
			out = append(out, b[i])
			i++
			continue
		}
		next := b[i+1]
		switch next {
		case '[': // CSI — skip param/intermediate bytes then one final byte
			j := i + 2
			// param bytes: 0x30–0x3F
			for j < len(b) && b[j] >= 0x30 && b[j] <= 0x3F {
				j++
			}
			// intermediate bytes: 0x20–0x2F
			for j < len(b) && b[j] >= 0x20 && b[j] <= 0x2F {
				j++
			}
			// final byte: 0x40–0x7E
			if j < len(b) && b[j] >= 0x40 && b[j] <= 0x7E {
				j++
			}
			i = j
		case ']': // OSC — skip until BEL or ESC \
			j := i + 2
			for j < len(b) {
				if b[j] == 0x07 { // BEL
					j++
					break
				}
				if b[j] == 0x1b && j+1 < len(b) && b[j+1] == '\\' { // ST
					j += 2
					break
				}
				j++
			}
			i = j
		default: // any other two-byte ESC sequence
			i += 2
		}
	}
	return string(out)
}

// Snapshot returns the current rendered screen of the process as plain text.
// It works for both running and finished processes, and correctly handles TUI
// applications by replaying their escape sequences through a VT100 emulator.
func (pm *ProcessManager) Snapshot(id int) (string, error) {
	pm.mu.RLock()
	mp, ok := pm.processes[id]
	pm.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("process %d not found", id)
	}
	return renderScreen(mp.vt), nil
}

// stripVTProblematic removes CSI sequences that vt10x misinterprets due to
// non-digit parameter prefix bytes ('>','<','='). These are kitty keyboard
// protocol sequences (CSI > n u, CSI < u, CSI > n m, CSI > n q) which have
// no visual effect but cause vt10x to call restoreCursor(), jumping the cursor
// to (0,0) and corrupting subsequent rendering.
func stripVTProblematic(data []byte) []byte {
	out := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if i+1 < len(data) && data[i] == 0x1b && data[i+1] == '[' {
			j := i + 2
			if j < len(data) && (data[j] == '>' || data[j] == '<' || data[j] == '=') {
				// Skip to final byte (0x40–0x7E) inclusive
				for j < len(data) && (data[j] < 0x40 || data[j] > 0x7E) {
					j++
				}
				if j < len(data) {
					j++ // include final byte in skip
				}
				i = j
				continue
			}
		}
		out = append(out, data[i])
		i++
	}
	return out
}

// renderScreen renders the VT100 terminal state into clean plain text,
// trimming trailing whitespace per line and trailing blank lines.
func renderScreen(term vt10x.Terminal) string {
	raw := term.String() // String() locks the terminal internally
	lines := strings.Split(raw, "\n")

	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}

	// Drop trailing blank lines
	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}

	return strings.Join(lines[:end], "\n")
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
			if mp.info.Alias != "" {
				delete(pm.aliases, mp.info.Alias)
			}
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
