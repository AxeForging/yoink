package actions

import (
	"fmt"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// SnapshotAction renders the current VT100 screen of a process as plain text.
type SnapshotAction struct {
	client *services.ClientService
}

// NewSnapshotAction creates a SnapshotAction.
func NewSnapshotAction(client *services.ClientService) *SnapshotAction {
	return &SnapshotAction{client: client}
}

// Execute prints the rendered screen of the process to stdout.
func (a *SnapshotAction) Execute(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: yoink snapshot <id|alias>")
	}

	id, alias := parseProcessRef(c.Args().First())

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:  domain.ReqSnapshot,
		ID:    id,
		Alias: alias,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Print(resp.Output)
	if len(resp.Output) > 0 && resp.Output[len(resp.Output)-1] != '\n' {
		fmt.Println()
	}
	return nil
}
