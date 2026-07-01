package actions

import (
	"fmt"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// CleanAction removes finished/failed processes from the daemon.
type CleanAction struct {
	client *services.ClientService
}

// NewCleanAction creates a CleanAction.
func NewCleanAction(client *services.ClientService) *CleanAction {
	return &CleanAction{client: client}
}

// Execute removes non-running processes, or with --all, stops and removes everything.
func (a *CleanAction) Execute(c *cli.Context) error {
	all := c.Bool("all")

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:  domain.ReqClean,
		All:   all,
		Force: c.Bool("force"),
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	switch {
	case resp.Cleaned == 0:
		fmt.Println("Nothing to clean")
	case all:
		fmt.Printf("Stopped and removed %d process(es)\n", resp.Cleaned)
	default:
		fmt.Printf("Cleaned %d process(es)\n", resp.Cleaned)
	}
	return nil
}
