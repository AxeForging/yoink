package actions

import (
	"fmt"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// KillAction terminates a managed process.
type KillAction struct {
	client *services.ClientService
}

// NewKillAction creates a KillAction.
func NewKillAction(client *services.ClientService) *KillAction {
	return &KillAction{client: client}
}

// Execute sends SIGTERM (or SIGKILL with --force) to the process.
func (a *KillAction) Execute(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: yoink kill <id|alias> [--force]")
	}

	id, alias := parseProcessRef(c.Args().First())

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:  domain.ReqKill,
		ID:    id,
		Alias: alias,
		Force: c.Bool("force"),
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	sig := "SIGTERM"
	if c.Bool("force") {
		sig = "SIGKILL"
	}
	fmt.Printf("Sent %s to process %d\n", sig, id)
	return nil
}
