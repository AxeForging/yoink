package actions

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// LsAction lists all managed background processes.
type LsAction struct {
	client *services.ClientService
}

// NewLsAction creates a LsAction.
func NewLsAction(client *services.ClientService) *LsAction {
	return &LsAction{client: client}
}

// Execute lists all processes with their state.
func (a *LsAction) Execute(c *cli.Context) error {
	resp, err := a.client.Do(domain.DaemonRequest{Type: domain.ReqList})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	if len(resp.Processes) == 0 {
		fmt.Println("No managed processes")
		return nil
	}

	sort.Slice(resp.Processes, func(i, j int) bool {
		return resp.Processes[i].ID < resp.Processes[j].ID
	})

	full := c.Bool("full")

	header := []string{"ID", "ALIAS", "STATE", "PID", "EXIT", "STARTED", "COMMAND"}
	if full {
		header = []string{"ID", "ALIAS", "STATE", "PID", "EXIT", "STARTED", "CWD", "COMMAND"}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, p := range resp.Processes {
		exitStr := "-"
		if p.State != domain.StateRunning {
			exitStr = fmt.Sprintf("%d", p.ExitCode)
		}
		aliasStr := "-"
		if p.Alias != "" {
			aliasStr = p.Alias
		}
		command := p.Command
		if !full {
			command = truncate(command, 60)
		}

		row := []string{
			fmt.Sprintf("%d", p.ID),
			aliasStr,
			string(p.State),
			fmt.Sprintf("%d", p.PID),
			exitStr,
			p.StartedAt.Format("15:04:05"),
		}
		if full {
			cwdStr := p.Cwd
			if cwdStr == "" {
				cwdStr = "-"
			}
			row = append(row, cwdStr)
		}
		row = append(row, command)

		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
