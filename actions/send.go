package actions

import (
	"fmt"
	"strings"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// resolveInput returns the input string and whether no-enter should be implied,
// handling both plain text and --key mode.
func resolveInput(c *cli.Context) (input string, noEnter bool, err error) {
	keyName := c.String("key")
	hasText := c.NArg() >= 2

	if keyName != "" && hasText {
		return "", false, fmt.Errorf("--key and text input are mutually exclusive")
	}
	if keyName != "" {
		seq, err := services.ResolveKey(keyName)
		if err != nil {
			return "", false, err
		}
		return seq, true, nil // key sequences never get a trailing newline
	}
	return strings.Join(c.Args()[1:], " "), c.Bool("no-enter"), nil
}

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
	keyName := c.String("key")
	if c.NArg() < 1 || (c.NArg() < 2 && keyName == "") {
		cmd := "send"
		if a.redacted {
			cmd = "send-redacted"
		}
		return fmt.Errorf("usage: yoink %s <id|alias> <input> [--key <name>]", cmd)
	}

	id, alias := parseProcessRef(c.Args().First())

	input, noEnter, err := resolveInput(c)
	if err != nil {
		return err
	}

	resp, err := a.client.Do(domain.DaemonRequest{
		Type:      domain.ReqSend,
		ID:        id,
		Alias:     alias,
		Input:     input,
		Redacted:  a.redacted,
		TypeMode:  c.Bool("type"),
		TypeDelay: c.Int("delay"),
		NoEnter:   noEnter,
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
