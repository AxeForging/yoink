package actions

import (
	"fmt"
	"strings"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// SendAction sends input text to a running process.
type SendAction struct {
	client   *services.ClientService
	redacted bool
}

// NewSendAction creates a SendAction.
func NewSendAction(client *services.ClientService, redacted bool) *SendAction {
	return &SendAction{client: client, redacted: redacted}
}

// Execute sends the given text as stdin to the process.
func (a *SendAction) Execute(c *cli.Context) error {
	if c.NArg() < 2 {
		cmd := "send"
		if a.redacted {
			cmd = "send-redacted"
		}
		return fmt.Errorf("usage: yoink %s <id|alias> <input>", cmd)
	}

	id, alias := parseProcessRef(c.Args().First())
	input := strings.Join(c.Args()[1:], " ")

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:      domain.ReqSend,
		ID:        id,
		Alias:     alias,
		Input:     input,
		Redacted:  a.redacted,
		TypeMode:  c.Bool("type"),
		TypeDelay: c.Int("delay"),
		NoEnter:   c.Bool("no-enter"),
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	if a.redacted {
		fmt.Printf("Sent redacted input to process %d\n", id)
	} else {
		fmt.Printf("Sent to process %d: %s\n", id, input)
	}
	return nil
}
