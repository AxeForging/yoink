//go:build windows

package services

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// StartProcessWithPTY is not yet supported on Windows.
func StartProcessWithPTY(cmd *exec.Cmd) (*os.File, error) {
	return nil, fmt.Errorf("PTY support is not available on Windows yet")
}

// DaemonSysProcAttr returns SysProcAttr for daemonizing on Windows.
func DaemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

// TerminateProcess kills the process on Windows (no SIGTERM equivalent).
func TerminateProcess(p *os.Process) error {
	return p.Kill()
}

// KillProcess kills the process on Windows.
func KillProcess(p *os.Process) error {
	return p.Kill()
}
