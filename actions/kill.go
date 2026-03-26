package actions

import (
	"fmt"
	"strconv"

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
		return fmt.Errorf("usage: yoink kill <id> [--force]")
	}

	id, err := strconv.Atoi(c.Args().First())
	if err != nil {
		return fmt.Errorf("invalid process ID: %s", c.Args().First())
	}

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:  domain.ReqKill,
		ID:    id,
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
