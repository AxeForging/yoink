package actions

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
	"os"
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tALIAS\tSTATE\tPID\tEXIT\tSTARTED\tCOMMAND")
	for _, p := range resp.Processes {
		exitStr := "-"
		if p.State != domain.StateRunning {
			exitStr = fmt.Sprintf("%d", p.ExitCode)
		}
		aliasStr := "-"
		if p.Alias != "" {
			aliasStr = p.Alias
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\t%s\n",
			p.ID, aliasStr, p.State, p.PID, exitStr,
			p.StartedAt.Format("15:04:05"),
			truncate(p.Command, 60),
		)
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
