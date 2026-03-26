//go:build !windows

package services

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// StartProcessWithPTY starts the command with a pseudo-terminal attached.
func StartProcessWithPTY(cmd *exec.Cmd) (*os.File, error) {
	return pty.Start(cmd)
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
