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

// Execute removes all non-running processes.
func (a *CleanAction) Execute(c *cli.Context) error {
	resp, err := a.client.Do(domain.DaemonRequest{Type: domain.ReqClean})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	if resp.Cleaned == 0 {
		fmt.Println("Nothing to clean")
	} else {
		fmt.Printf("Cleaned %d process(es)\n", resp.Cleaned)
	}
	return nil
}
