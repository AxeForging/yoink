//go:build windows

package services

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// lineEnding is what a plain `send` appends. ConPTY maps only CR to the Enter
// key; a bare LF is read as Ctrl+J and never completes a cooked-mode line.
const lineEnding = "\r"

// ShellCommand runs command through cmd.exe (honouring %COMSPEC%). The raw
// command line is passed verbatim with /s /c "..." so cmd's own quoting rules
// apply, instead of Go's argv escaping, which cmd.exe does not understand.
func ShellCommand(command string) *exec.Cmd {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}
	cmd := exec.Command(shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: syscall.EscapeArg(shell) + ` /d /s /c "` + command + `"`,
	}
	return cmd
}

// conPTY is a Windows pseudo console: Read yields the rendered VT output,
// Write feeds keyboard input. The child runs inside a Job Object so its whole
// tree can be killed without ever touching a recycled PID.
type conPTY struct {
	hpc       windows.Handle
	job       windows.Handle
	proc      *os.Process
	in        *os.File // our end of the console's input pipe
	out       *os.File // our end of the console's output pipe
	closeOnce sync.Once
}

// jobs maps a started process to its Job Object for KillProcess.
var jobs sync.Map // *os.Process -> windows.Handle

func (c *conPTY) Read(p []byte) (int, error)  { return c.out.Read(p) }
func (c *conPTY) Write(p []byte) (int, error) { return c.in.Write(p) }

// ProcessExited closes the pseudo console. ConPTY keeps its output pipe open
// after the child exits, so without this the output reader never sees EOF.
// Closing flushes the final frame into the pipe first.
func (c *conPTY) ProcessExited() {
	c.closeOnce.Do(func() { windows.ClosePseudoConsole(c.hpc) })
}

// Close releases the console and the job. The job is kill-on-close, so any
// leftover descendants die with it, as a PTY hangup does on Unix.
func (c *conPTY) Close() error {
	c.ProcessExited()
	if c.proc != nil {
		jobs.Delete(c.proc)
	}
	if c.job != 0 {
		_ = windows.CloseHandle(c.job)
	}
	_ = c.in.Close()
	return c.out.Close()
}

// newKillOnCloseJob creates a Job Object that kills its processes when its
// last handle closes, so a crashed daemon cannot leak children.
func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// StartProcessWithPTY starts cmd attached to a ConPTY sized to match the VT
// emulator in process_manager.go (220×50). It sets cmd.Process so callers can
// Wait and signal it as on Unix.
// shortcut: cmd.Env is ignored (the child inherits the daemon env); Spawn never sets it.
func StartProcessWithPTY(cmd *exec.Cmd) (_ *conPTY, err error) {
	var inRead, inWrite, outRead, outWrite windows.Handle
	if err := windows.CreatePipe(&inRead, &inWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("create input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outRead, &outWrite, nil, 0); err != nil {
		windows.CloseHandle(inRead)  //nolint:errcheck
		windows.CloseHandle(inWrite) //nolint:errcheck
		return nil, fmt.Errorf("create output pipe: %w", err)
	}
	// The console owns its pipe ends once created; we only keep ours.
	defer windows.CloseHandle(inRead)   //nolint:errcheck
	defer windows.CloseHandle(outWrite) //nolint:errcheck

	c := &conPTY{
		in:  os.NewFile(uintptr(inWrite), "conpty-in"),
		out: os.NewFile(uintptr(outRead), "conpty-out"),
	}
	if err := windows.CreatePseudoConsole(windows.Coord{X: 220, Y: 50}, inRead, outWrite, 0, &c.hpc); err != nil {
		_ = c.in.Close()
		_ = c.out.Close()
		return nil, fmt.Errorf("create pseudo console: %w", err)
	}
	defer func() {
		if err != nil {
			_ = c.Close()
		}
	}()
	if c.job, err = newKillOnCloseJob(); err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, fmt.Errorf("allocate attribute list: %w", err)
	}
	defer attrs.Delete()
	// The attribute value is the HPCON itself, not a pointer to it.
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, *(*unsafe.Pointer)(unsafe.Pointer(&c.hpc)), unsafe.Sizeof(c.hpc)); err != nil {
		return nil, fmt.Errorf("attach pseudo console: %w", err)
	}

	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(si))
	// Null std handles force the child onto the pseudo console. Without this a
	// daemon whose own stdout is redirected to a log file leaks that handle.
	si.Flags = windows.STARTF_USESTDHANDLES

	cmdLine := cmd.Path
	if cmd.SysProcAttr != nil && cmd.SysProcAttr.CmdLine != "" {
		cmdLine = cmd.SysProcAttr.CmdLine
	}
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return nil, err
	}
	var dirPtr *uint16
	if cmd.Dir != "" {
		if dirPtr, err = windows.UTF16PtrFromString(cmd.Dir); err != nil {
			return nil, err
		}
	}

	// Start suspended so the process is in the job before it can spawn children.
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(nil, cmdLinePtr, nil, nil, false,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_SUSPENDED, nil, dirPtr, &si.StartupInfo, &pi); err != nil {
		return nil, fmt.Errorf("create process: %w", err)
	}
	defer windows.CloseHandle(pi.Thread) //nolint:errcheck
	// Our handle keeps the PID valid until FindProcess has opened its own.
	defer windows.CloseHandle(pi.Process) //nolint:errcheck

	if err := windows.AssignProcessToJobObject(c.job, pi.Process); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		return nil, fmt.Errorf("assign job object: %w", err)
	}
	if cmd.Process, err = os.FindProcess(int(pi.ProcessId)); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		return nil, fmt.Errorf("open process: %w", err)
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		return nil, fmt.Errorf("resume process: %w", err)
	}
	c.proc = cmd.Process
	jobs.Store(cmd.Process, c.job)
	return c, nil
}

// DaemonSysProcAttr detaches the daemon from the caller's console so closing
// that terminal (or Ctrl+C in it) does not take the daemon down.
func DaemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
}

// TerminateProcess kills the process tree. Windows has no SIGTERM for console
// programs, and cmd.exe does not exec its child, so killing only the shell
// would orphan the real workload.
func TerminateProcess(p *os.Process) error {
	return KillProcess(p)
}

// KillProcess force-kills the process and all its descendants via its job.
func KillProcess(p *os.Process) error {
	if job, ok := jobs.Load(p); ok {
		return windows.TerminateJobObject(job.(windows.Handle), 1)
	}
	return p.Kill()
}
