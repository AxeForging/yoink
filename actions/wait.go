package actions

import (
	"fmt"
	"os"
	"time"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// WaitAction polls a background process until it finishes or a timeout is reached,
// streaming new output to stdout on each poll cycle.
type WaitAction struct {
	client *services.ClientService
}

// NewWaitAction creates a WaitAction.
func NewWaitAction(client *services.ClientService) *WaitAction {
	return &WaitAction{client: client}
}

// Execute polls the process identified by ID or alias, streaming new output and
// exiting with the process exit code on completion or 124 on timeout.
func (a *WaitAction) Execute(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: yoink wait <id|alias> [--timeout SECONDS] [--poll SECONDS] [--message TEXT]")
	}

	id, alias := parseProcessRef(c.Args().First())

	timeout := c.Int("timeout")
	poll := c.Int("poll")
	if poll <= 0 {
		poll = 2
	}
	message := c.String("message")

	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(time.Duration(timeout) * time.Second)
	}

	for {
		// Drain new output to stdout
		if err := a.drainOutput(id, alias); err != nil {
			return err
		}

		// Check process state
		proc, err := a.findProcess(id, alias)
		if err != nil {
			return err
		}

		// Resolve ID from first successful lookup (for alias-based refs)
		if id == 0 && proc.ID > 0 {
			id = proc.ID
		}

		switch proc.State {
		case domain.StateDone:
			return nil
		case domain.StateFailed:
			return cli.NewExitError(
				fmt.Sprintf("process exited with code %d", proc.ExitCode),
				proc.ExitCode,
			)
		}

		// Check timeout
		if timeout > 0 && time.Now().After(deadline) {
			msg := message
			if msg == "" {
				msg = fmt.Sprintf("timeout: process did not finish within %ds", timeout)
			}
			return cli.NewExitError(msg, 124)
		}

		time.Sleep(time.Duration(poll) * time.Second)
	}
}

func (a *WaitAction) drainOutput(id int, alias string) error {
	req := domain.DaemonRequest{
		Type:  domain.ReqLog,
		ID:    id,
		Alias: alias,
		New:   true,
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	if resp.Output != "" {
		fmt.Fprintln(os.Stdout, resp.Output)
	}
	return nil
}

func (a *WaitAction) findProcess(id int, alias string) (*domain.ProcessInfo, error) {
	resp, err := a.client.Do(domain.DaemonRequest{Type: domain.ReqList})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	for _, p := range resp.Processes {
		if (id > 0 && p.ID == id) || (alias != "" && p.Alias == alias) {
			return &p, nil
		}
	}

	ref := alias
	if ref == "" {
		ref = fmt.Sprintf("%d", id)
	}
	return nil, fmt.Errorf("process %s not found", ref)
}
