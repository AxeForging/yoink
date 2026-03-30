package actions

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// WaitAction polls one or more background processes until they finish or a
// timeout is reached, streaming new output to stdout on each poll cycle.
type WaitAction struct {
	client *services.ClientService
}

// NewWaitAction creates a WaitAction.
func NewWaitAction(client *services.ClientService) *WaitAction {
	return &WaitAction{client: client}
}

// waitTarget tracks a single process being waited on.
type waitTarget struct {
	id    int
	alias string
	label string // display label (alias or id)
	done  bool
}

// Execute polls processes identified by IDs or aliases, streaming new output and
// exiting with the process exit code on failure or 124 on timeout.
func (a *WaitAction) Execute(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: yoink wait <id|alias>... [--timeout SECONDS] [--poll SECONDS] [--message TEXT]")
	}

	targets := make([]waitTarget, c.NArg())
	for i := 0; i < c.NArg(); i++ {
		ref := c.Args().Get(i)
		id, alias := parseProcessRef(ref)
		targets[i] = waitTarget{id: id, alias: alias, label: ref}
	}

	multi := len(targets) > 1

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
		// Fetch process list once per cycle (shared across all targets)
		processes, err := a.listProcesses()
		if err != nil {
			return err
		}

		allDone := true
		for i := range targets {
			t := &targets[i]
			if t.done {
				continue
			}

			// Drain new output
			if err := a.drainOutput(t.id, t.alias, t.label, multi); err != nil {
				return err
			}

			// Find process in list
			proc := findInList(processes, t.id, t.alias)
			if proc == nil {
				return fmt.Errorf("process %s not found", t.label)
			}

			// Resolve ID on first lookup (for alias-based refs)
			if t.id == 0 && proc.ID > 0 {
				t.id = proc.ID
			}

			switch proc.State {
			case domain.StateDone:
				t.done = true
			case domain.StateFailed:
				return cli.NewExitError(
					fmt.Sprintf("process %s exited with code %d", t.label, proc.ExitCode),
					proc.ExitCode,
				)
			default:
				allDone = false
			}
		}

		if allDone {
			return nil
		}

		// Check timeout
		if timeout > 0 && time.Now().After(deadline) {
			msg := message
			if msg == "" {
				pending := pendingLabels(targets)
				msg = fmt.Sprintf("timeout: processes did not finish within %ds: %s", timeout, strings.Join(pending, ", "))
			}
			return cli.NewExitError(msg, 124)
		}

		time.Sleep(time.Duration(poll) * time.Second)
	}
}

func (a *WaitAction) drainOutput(id int, alias, label string, multi bool) error {
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
		if multi {
			for _, line := range strings.Split(resp.Output, "\n") {
				fmt.Fprintf(os.Stdout, "[%s] %s\n", label, line)
			}
		} else {
			fmt.Fprintln(os.Stdout, resp.Output)
		}
	}
	return nil
}

func (a *WaitAction) listProcesses() ([]domain.ProcessInfo, error) {
	resp, err := a.client.Do(domain.DaemonRequest{Type: domain.ReqList})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Processes, nil
}

func findInList(processes []domain.ProcessInfo, id int, alias string) *domain.ProcessInfo {
	for _, p := range processes {
		if (id > 0 && p.ID == id) || (alias != "" && p.Alias == alias) {
			return &p
		}
	}
	return nil
}

func pendingLabels(targets []waitTarget) []string {
	var labels []string
	for _, t := range targets {
		if !t.done {
			labels = append(labels, t.label)
		}
	}
	return labels
}
