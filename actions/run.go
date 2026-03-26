package actions

import (
	"fmt"
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

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:    domain.ReqRun,
		Command: command,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Printf("Started process %d: %s\n", resp.ID, command)
	return nil
}
