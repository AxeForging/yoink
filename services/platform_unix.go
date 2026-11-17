//go:build !windows

package services

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// lineEnding is what a plain `send` appends; the tty line discipline turns it
// into Enter for cooked-mode readers.
const lineEnding = "\n"

// ShellCommand runs command through sh so pipes, quoting and env expansion work.
func ShellCommand(command string) *exec.Cmd {
	return exec.Command("sh", "-c", command)
}

// StartProcessWithPTY starts the command with a pseudo-terminal attached.
// The PTY size is set to match the VT emulator size in process_manager.go (220×50).
func StartProcessWithPTY(cmd *exec.Cmd) (*os.File, error) {
	return pty.StartWithSize(cmd, &pty.Winsize{Rows: 50, Cols: 220})
}

// DaemonSysProcAttr returns SysProcAttr for daemonizing on Unix (new session).
func DaemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// TerminateProcess sends SIGTERM to the process.
func TerminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}

// KillProcess sends SIGKILL to the process.
func KillProcess(p *os.Process) error {
	return p.Signal(syscall.SIGKILL)
}
