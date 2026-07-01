package domain

import "time"

// ProcessState represents the current state of a managed process.
type ProcessState string

const (
	StateRunning ProcessState = "running"
	StateDone    ProcessState = "done"
	StateFailed  ProcessState = "failed"
)

// ProcessInfo holds metadata about a managed background process.
type ProcessInfo struct {
	ID         int          `json:"id"`
	Alias      string       `json:"alias,omitempty"`
	Command    string       `json:"command"`
	State      ProcessState `json:"state"`
	PID        int          `json:"pid"`
	ExitCode   int          `json:"exit_code"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	Cwd        string       `json:"cwd,omitempty"`
	LogFile    string       `json:"log_file,omitempty"`
}

// LineSource identifies where a log line came from.
type LineSource string

const (
	SourceStdout LineSource = "out"
	SourceStdin  LineSource = "in"
)

// LogEntry is a single line in the process output buffer.
type LogEntry struct {
	Source   LineSource `json:"source"`
	Text     string     `json:"text"`
	Redacted bool       `json:"redacted,omitempty"`
}

// RequestType identifies the kind of daemon request.
type RequestType string

const (
	ReqRun      RequestType = "run"
	ReqList     RequestType = "list"
	ReqLog      RequestType = "log"
	ReqSend     RequestType = "send"
	ReqAttach   RequestType = "attach"
	ReqKill     RequestType = "kill"
	ReqClean    RequestType = "clean"
	ReqShutdown RequestType = "shutdown"
	ReqSnapshot RequestType = "snapshot"
)

// DaemonRequest is sent from client to daemon over the Unix socket.
type DaemonRequest struct {
	Type      RequestType `json:"type"`
	ID        int         `json:"id,omitempty"`
	Alias     string      `json:"alias,omitempty"`
	Command   string      `json:"command,omitempty"`
	Input     string      `json:"input,omitempty"`
	Lines     int         `json:"lines,omitempty"`
	New       bool        `json:"new,omitempty"`
	Force     bool        `json:"force,omitempty"`
	All       bool        `json:"all,omitempty"`
	Redacted  bool        `json:"redacted,omitempty"`
	TypeMode  bool        `json:"type_mode,omitempty"`
	TypeDelay int         `json:"type_delay,omitempty"` // milliseconds between characters
	NoEnter   bool        `json:"no_enter,omitempty"`
	Submit    bool        `json:"submit,omitempty"`
	Cwd       string      `json:"cwd,omitempty"`
	LogFile   string      `json:"log_file,omitempty"`
}

// DaemonResponse is sent from daemon to client over the Unix socket.
type DaemonResponse struct {
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	ID        int           `json:"id,omitempty"`
	Processes []ProcessInfo `json:"processes,omitempty"`
	Output    string        `json:"output,omitempty"`
	Cleaned   int           `json:"cleaned,omitempty"`
}
