package actions

import (
	"fmt"
	"strconv"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// LogAction shows recent output from a managed process.
type LogAction struct {
	client *services.ClientService
}

// NewLogAction creates a LogAction.
func NewLogAction(client *services.ClientService) *LogAction {
	return &LogAction{client: client}
}

// Execute prints the last N lines of output for the given process.
func (a *LogAction) Execute(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: yoink log <id> [--lines N]")
	}

	id, err := strconv.Atoi(c.Args().First())
	if err != nil {
		return fmt.Errorf("invalid process ID: %s", c.Args().First())
	}

	lines := c.Int("lines")
	if lines <= 0 {
		lines = 50
	}

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    id,
		Lines: lines,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	if resp.Output != "" {
		fmt.Println(resp.Output)
	}
	return nil
}
