package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// RunAction spawns a command in the background via the daemon.
type RunAction struct {
	client *services.ClientService
}

// NewRunAction creates a RunAction.
func NewRunAction(client *services.ClientService) *RunAction {
	return &RunAction{client: client}
}

// Execute spawns the given command and prints the process ID.
func (a *RunAction) Execute(c *cli.Context) error {
	command := strings.Join(c.Args(), " ")
	if command == "" {
		return fmt.Errorf("usage: yoink run <command>")
	}

	alias := c.String("alias")

	// The daemon is a long-lived background process that may have been
	// auto-started from an unrelated directory at some point in the past, so
	// its own cwd is not a reliable default. Always resolve the process's
	// working directory here, against the caller's actual cwd.
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine current directory: %w", err)
	}

	cwd := c.String("cwd")
	if cwd == "" {
		cwd = wd
	} else if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(wd, cwd)
	}

	logFile := c.String("log-file")
	if logFile != "" && !filepath.IsAbs(logFile) {
		logFile = filepath.Join(wd, logFile)
	}

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: command,
		Alias:   alias,
		Cwd:     cwd,
		LogFile: logFile,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	if alias != "" {
		fmt.Printf("Started process %d (%s): %s\n", resp.ID, alias, command)
	} else {
		fmt.Printf("Started process %d: %s\n", resp.ID, command)
	}
	return nil
}
