package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/AxeForging/yoink/domain"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Wait bounds. MCP clients cap how long a single tool call may run (commonly
// around 60s), so yoink_wait returns a resumable "still running" result rather
// than blocking past a limit the client would kill anyway.
const (
	defaultWaitSeconds = 30
	maxWaitSeconds     = 55
	defaultPollSeconds = 2
	defaultLogLines    = 50
)

// Tool input types. Field tags drive the JSON Schema the SDK advertises to
// clients, so the descriptions here are the agent-facing docs.

type mcpRunInput struct {
	Command string `json:"command" jsonschema:"the command line to run under a managed PTY"`
	Alias   string `json:"alias,omitempty" jsonschema:"short name to reference this process by later; strongly recommended"`
	Cwd     string `json:"cwd,omitempty" jsonschema:"working directory for the process; relative paths resolve against the MCP server's directory"`
	LogFile string `json:"logFile,omitempty" jsonschema:"also tee the full raw output to this file, unbounded by the 1000-line ring buffer"`
}

type mcpProcessInput struct {
	Process string `json:"process" jsonschema:"the process to target, by numeric id or alias"`
}

type mcpLogInput struct {
	Process string `json:"process" jsonschema:"the process to read, by numeric id or alias"`
	Lines   int    `json:"lines,omitempty" jsonschema:"how many trailing lines to return (default 50); ignored when new is true"`
	New     bool   `json:"new,omitempty" jsonschema:"return only lines produced since the last new=true read of this process; the preferred way to poll"`
}

type mcpSendInput struct {
	Process     string `json:"process" jsonschema:"the process to send to, by numeric id or alias"`
	Input       string `json:"input,omitempty" jsonschema:"text to type into the process; mutually exclusive with key"`
	Key         string `json:"key,omitempty" jsonschema:"named key sequence to send instead of text, e.g. enter, up, ctrl+c, escape, f1; mutually exclusive with input"`
	Redacted    bool   `json:"redacted,omitempty" jsonschema:"mask this input in the process log; use for passwords and tokens"`
	Submit      bool   `json:"submit,omitempty" jsonschema:"terminate the input with a carriage return instead of a newline; TUI prompts usually need this"`
	NoEnter     bool   `json:"noEnter,omitempty" jsonschema:"send the text without any trailing newline"`
	Type        bool   `json:"type,omitempty" jsonschema:"simulate human typing, one character at a time"`
	TypeDelayMs int    `json:"typeDelayMs,omitempty" jsonschema:"milliseconds between characters when type is true"`
}

type mcpWaitInput struct {
	Processes      []string `json:"processes" jsonschema:"the processes to wait on, by numeric id or alias"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty" jsonschema:"how long to wait before returning a resumable timedOut result (default 30, max 55)"`
	PollSeconds    int      `json:"pollSeconds,omitempty" jsonschema:"seconds between state checks (default 2)"`
}

type mcpKillInput struct {
	Process string `json:"process" jsonschema:"the process to terminate, by numeric id or alias"`
	Force   bool   `json:"force,omitempty" jsonschema:"send SIGKILL instead of SIGTERM"`
}

type mcpCleanInput struct {
	All   bool `json:"all,omitempty" jsonschema:"also stop and remove processes that are still running"`
	Force bool `json:"force,omitempty" jsonschema:"with all, use SIGKILL instead of SIGTERM"`
}

type mcpEmptyInput struct{}

// Tool output types.

type mcpRunOutput struct {
	ID    int    `json:"id"`
	Alias string `json:"alias,omitempty"`
	Cwd   string `json:"cwd"`
}

type mcpListOutput struct {
	Processes []domain.ProcessInfo `json:"processes"`
}

type mcpLogOutput struct {
	Output string `json:"output"`
}

type mcpSendOutput struct {
	ID int `json:"id"`
}

type mcpSnapshotOutput struct {
	Screen string `json:"screen"`
}

type mcpKillOutput struct {
	ID     int    `json:"id"`
	Signal string `json:"signal"`
}

type mcpCleanOutput struct {
	Cleaned int `json:"cleaned"`
}

// mcpWaitProcess is the per-process outcome of a wait.
type mcpWaitProcess struct {
	Process  string `json:"process"`
	ID       int    `json:"id,omitempty"`
	State    string `json:"state"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Output   string `json:"output,omitempty"`
}

type mcpWaitOutput struct {
	Done           bool             `json:"done"`
	TimedOut       bool             `json:"timedOut"`
	Failed         bool             `json:"failed"`
	TimeoutSeconds int              `json:"timeoutSeconds"`
	Processes      []mcpWaitProcess `json:"processes"`
}

// addTool registers one tool. The SDK's AddTool is generic over the tool's
// input and output types, so handlers cannot be collected into a table; this
// keeps each registration to a single line instead.
func addTool[In, Out any](srv *mcpsdk.Server, name, description string, handler mcpsdk.ToolHandlerFor[In, Out]) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: name, Description: description}, handler)
}

// registerTools wires every tool to its handler on the SDK server. The
// descriptions are the agent-facing docs — they are how a model learns that
// log(new) is the way to poll and that snapshot, not log, is for TUIs.
func (s *MCPServer) registerTools(srv *mcpsdk.Server) {
	addTool(srv, "yoink_run",
		"Start a command in the background under a real PTY and return immediately with its process id. Use this instead of a blocking shell call for dev servers, builds, test watchers, deploys, and any interactive CLI. Always pass an alias.",
		s.handleRun)

	addTool(srv, "yoink_ls",
		"List every process the yoink daemon is tracking, with state, pid, exit code, working directory, and log file. Includes processes started from a human's terminal, not just ones started through MCP.",
		s.handleList)

	addTool(srv, "yoink_log",
		"Read a process's output. Prefer new=true when polling: it returns only the lines produced since your last new=true read, so repeated calls never re-read the same output. Output is a 1000-line ring buffer; for full output, start the process with logFile.",
		s.handleLog)

	addTool(srv, "yoink_send",
		"Send text or a named key to a running process's stdin — answer a prompt, confirm a y/n, drive an interactive CLI. Use redacted for secrets, and submit for TUI prompts that expect a carriage return.",
		s.handleSend)

	addTool(srv, "yoink_snapshot",
		"Render a process's current terminal screen as plain text. Use this instead of yoink_log for full-screen TUI programs (vim, htop, ncurses apps), where raw logs are unreadable escape sequences.",
		s.handleSnapshot)

	addTool(srv, "yoink_wait",
		"Block until the given processes finish, returning their exit codes and any new output. Bounded and resumable: if they are still running when timeoutSeconds elapses this returns done=false and timedOut=true rather than failing, and you can simply call again to keep waiting.",
		s.handleWait)

	addTool(srv, "yoink_kill",
		"Terminate a running process with SIGTERM, or SIGKILL when force is set.",
		s.handleKill)

	addTool(srv, "yoink_clean",
		"Remove finished and failed processes from the list. With all=true, also stop and remove processes that are still running. Call this when you are done with a set of background processes.",
		s.handleClean)
}

func (s *MCPServer) handleRun(_ context.Context, _ *mcpsdk.CallToolRequest, in mcpRunInput) (*mcpsdk.CallToolResult, mcpRunOutput, error) {
	if in.Command == "" {
		return nil, mcpRunOutput{}, fmt.Errorf("command is required")
	}

	// The daemon is long-lived and may have been auto-started from an unrelated
	// directory, so its own cwd is never a usable default. Resolve here, exactly
	// as the CLI does.
	wd, err := os.Getwd()
	if err != nil {
		return nil, mcpRunOutput{}, fmt.Errorf("cannot determine current directory: %w", err)
	}

	cwd := in.Cwd
	if cwd == "" {
		cwd = wd
	} else if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(wd, cwd)
	}

	logFile := in.LogFile
	if logFile != "" && !filepath.IsAbs(logFile) {
		logFile = filepath.Join(wd, logFile)
	}

	resp, err := s.do(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: in.Command,
		Alias:   in.Alias,
		Cwd:     cwd,
		LogFile: logFile,
	})
	if err != nil {
		return nil, mcpRunOutput{}, err
	}
	return nil, mcpRunOutput{ID: resp.ID, Alias: in.Alias, Cwd: cwd}, nil
}

func (s *MCPServer) handleList(_ context.Context, _ *mcpsdk.CallToolRequest, _ mcpEmptyInput) (*mcpsdk.CallToolResult, mcpListOutput, error) {
	processes, err := s.listProcesses()
	if err != nil {
		return nil, mcpListOutput{}, err
	}
	return nil, mcpListOutput{Processes: processes}, nil
}

func (s *MCPServer) handleLog(_ context.Context, _ *mcpsdk.CallToolRequest, in mcpLogInput) (*mcpsdk.CallToolResult, mcpLogOutput, error) {
	id, alias, err := resolveRef(in.Process)
	if err != nil {
		return nil, mcpLogOutput{}, err
	}

	req := domain.DaemonRequest{Type: domain.ReqLog, ID: id, Alias: alias}
	if in.New {
		req.New = true
	} else {
		lines := in.Lines
		if lines <= 0 {
			lines = defaultLogLines
		}
		req.Lines = lines
	}

	resp, err := s.do(req)
	if err != nil {
		return nil, mcpLogOutput{}, err
	}
	return nil, mcpLogOutput{Output: resp.Output}, nil
}

func (s *MCPServer) handleSend(_ context.Context, _ *mcpsdk.CallToolRequest, in mcpSendInput) (*mcpsdk.CallToolResult, mcpSendOutput, error) {
	id, alias, err := resolveRef(in.Process)
	if err != nil {
		return nil, mcpSendOutput{}, err
	}

	input, noEnter, submit, err := resolveSendInput(in)
	if err != nil {
		return nil, mcpSendOutput{}, err
	}

	resp, err := s.do(domain.DaemonRequest{
		Type:      domain.ReqSend,
		ID:        id,
		Alias:     alias,
		Input:     input,
		Redacted:  in.Redacted,
		TypeMode:  in.Type,
		TypeDelay: in.TypeDelayMs,
		NoEnter:   noEnter,
		Submit:    submit,
	})
	if err != nil {
		return nil, mcpSendOutput{}, err
	}
	if resp.ID != 0 {
		id = resp.ID
	}
	return nil, mcpSendOutput{ID: id}, nil
}

// resolveSendInput validates the text/key combination and returns the bytes to
// send along with the trailing-newline behavior, mirroring the CLI's rules.
func resolveSendInput(in mcpSendInput) (input string, noEnter, submit bool, err error) {
	switch {
	case in.Key != "" && in.Input != "":
		return "", false, false, fmt.Errorf("input and key are mutually exclusive")
	case in.Key == "" && in.Input == "":
		return "", false, false, fmt.Errorf("one of input or key is required")
	case in.NoEnter && in.Submit:
		return "", false, false, fmt.Errorf("noEnter and submit are mutually exclusive")
	}

	if in.Key != "" {
		if in.Submit {
			return "", false, false, fmt.Errorf("key and submit are mutually exclusive")
		}
		seq, err := ResolveKey(in.Key)
		if err != nil {
			return "", false, false, err
		}
		return seq, true, false, nil // key sequences never get a trailing newline
	}
	return in.Input, in.NoEnter, in.Submit, nil
}

func (s *MCPServer) handleSnapshot(_ context.Context, _ *mcpsdk.CallToolRequest, in mcpProcessInput) (*mcpsdk.CallToolResult, mcpSnapshotOutput, error) {
	id, alias, err := resolveRef(in.Process)
	if err != nil {
		return nil, mcpSnapshotOutput{}, err
	}
	resp, err := s.do(domain.DaemonRequest{Type: domain.ReqSnapshot, ID: id, Alias: alias})
	if err != nil {
		return nil, mcpSnapshotOutput{}, err
	}
	return nil, mcpSnapshotOutput{Screen: resp.Output}, nil
}

func (s *MCPServer) handleKill(_ context.Context, _ *mcpsdk.CallToolRequest, in mcpKillInput) (*mcpsdk.CallToolResult, mcpKillOutput, error) {
	id, alias, err := resolveRef(in.Process)
	if err != nil {
		return nil, mcpKillOutput{}, err
	}
	resp, err := s.do(domain.DaemonRequest{Type: domain.ReqKill, ID: id, Alias: alias, Force: in.Force})
	if err != nil {
		return nil, mcpKillOutput{}, err
	}
	if resp.ID != 0 {
		id = resp.ID
	}
	signal := "SIGTERM"
	if in.Force {
		signal = "SIGKILL"
	}
	return nil, mcpKillOutput{ID: id, Signal: signal}, nil
}

func (s *MCPServer) handleClean(_ context.Context, _ *mcpsdk.CallToolRequest, in mcpCleanInput) (*mcpsdk.CallToolResult, mcpCleanOutput, error) {
	resp, err := s.do(domain.DaemonRequest{Type: domain.ReqClean, All: in.All, Force: in.Force})
	if err != nil {
		return nil, mcpCleanOutput{}, err
	}
	return nil, mcpCleanOutput{Cleaned: resp.Cleaned}, nil
}

// waitTarget tracks one process across poll cycles of a single wait call.
type waitTarget struct {
	ref      string
	id       int
	alias    string
	state    domain.ProcessState
	exitCode *int
	output   string
	done     bool
}

func (s *MCPServer) handleWait(ctx context.Context, _ *mcpsdk.CallToolRequest, in mcpWaitInput) (*mcpsdk.CallToolResult, mcpWaitOutput, error) {
	if len(in.Processes) == 0 {
		return nil, mcpWaitOutput{}, fmt.Errorf("processes is required (one or more numeric ids or aliases)")
	}

	targets := make([]waitTarget, len(in.Processes))
	for i, ref := range in.Processes {
		id, alias, err := resolveRef(ref)
		if err != nil {
			return nil, mcpWaitOutput{}, err
		}
		targets[i] = waitTarget{ref: ref, id: id, alias: alias, state: domain.StateRunning}
	}

	timeout := in.TimeoutSeconds
	if timeout <= 0 {
		timeout = defaultWaitSeconds
	}
	if timeout > maxWaitSeconds {
		timeout = maxWaitSeconds
	}
	poll := in.PollSeconds
	if poll <= 0 {
		poll = defaultPollSeconds
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	for {
		processes, err := s.listProcesses()
		if err != nil {
			return nil, mcpWaitOutput{}, err
		}

		allDone := true
		for i := range targets {
			t := &targets[i]
			if t.done {
				continue
			}

			// Drain new output first, so a process that finishes this cycle still
			// reports the lines it produced on the way out.
			out, err := s.drainOutput(t.id, t.alias)
			if err != nil {
				return nil, mcpWaitOutput{}, err
			}
			t.output += out

			proc := findProcess(processes, t.id, t.alias)
			if proc == nil {
				return nil, mcpWaitOutput{}, fmt.Errorf("process %q not found", t.ref)
			}
			if t.id == 0 && proc.ID > 0 {
				t.id = proc.ID
			}

			t.state = proc.State
			if proc.State == domain.StateRunning {
				allDone = false
				continue
			}
			code := proc.ExitCode
			t.exitCode = &code
			t.done = true
		}

		if allDone {
			return nil, waitResult(targets, false, timeout), nil
		}
		if !time.Now().Before(deadline) {
			return nil, waitResult(targets, true, timeout), nil
		}

		select {
		case <-ctx.Done():
			return nil, mcpWaitOutput{}, ctx.Err()
		case <-time.After(time.Duration(poll) * time.Second):
		}
	}
}

// waitResult assembles the wait outcome from the polled targets.
func waitResult(targets []waitTarget, timedOut bool, timeout int) mcpWaitOutput {
	out := mcpWaitOutput{
		Done:           !timedOut,
		TimedOut:       timedOut,
		TimeoutSeconds: timeout,
		Processes:      make([]mcpWaitProcess, len(targets)),
	}
	for i, t := range targets {
		if t.state == domain.StateFailed {
			out.Failed = true
		}
		out.Processes[i] = mcpWaitProcess{
			Process:  t.ref,
			ID:       t.id,
			State:    string(t.state),
			ExitCode: t.exitCode,
			Output:   t.output,
		}
	}
	return out
}

// drainOutput returns the lines produced since the previous new-read.
func (s *MCPServer) drainOutput(id int, alias string) (string, error) {
	resp, err := s.do(domain.DaemonRequest{Type: domain.ReqLog, ID: id, Alias: alias, New: true})
	if err != nil {
		return "", err
	}
	if resp.Output == "" {
		return "", nil
	}
	return resp.Output + "\n", nil
}

// listProcesses returns the daemon's process list, sorted by id for stable output.
func (s *MCPServer) listProcesses() ([]domain.ProcessInfo, error) {
	resp, err := s.do(domain.DaemonRequest{Type: domain.ReqList})
	if err != nil {
		return nil, err
	}
	processes := resp.Processes
	if processes == nil {
		processes = []domain.ProcessInfo{}
	}
	sort.Slice(processes, func(i, j int) bool { return processes[i].ID < processes[j].ID })
	return processes, nil
}

func findProcess(processes []domain.ProcessInfo, id int, alias string) *domain.ProcessInfo {
	for i := range processes {
		p := &processes[i]
		if (id > 0 && p.ID == id) || (alias != "" && p.Alias == alias) {
			return p
		}
	}
	return nil
}
