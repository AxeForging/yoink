package actions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// SendAction sends input text to a running process.
type SendAction struct {
	client *services.ClientService
}

// NewSendAction creates a SendAction.
func NewSendAction(client *services.ClientService) *SendAction {
	return &SendAction{client: client}
}

// Execute sends the given text as stdin to the process.
func (a *SendAction) Execute(c *cli.Context) error {
	if c.NArg() < 2 {
		return fmt.Errorf("usage: yoink send <id> <input>")
	}

	id, err := strconv.Atoi(c.Args().First())
	if err != nil {
		return fmt.Errorf("invalid process ID: %s", c.Args().First())
	}

	input := strings.Join(c.Args()[1:], " ")

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:  domain.ReqSend,
		ID:    id,
		Input: input,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Printf("Sent to process %d: %s\n", id, input)
	return nil
}
