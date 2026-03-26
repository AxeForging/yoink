package actions

import (
	"fmt"

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
		return fmt.Errorf("usage: yoink log <id|alias> [--lines N]")
	}

	id, alias := parseProcessRef(c.Args().First())

	lines := c.Int("lines")
	if lines <= 0 {
		lines = 50
	}

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    id,
		Alias: alias,
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
