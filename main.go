package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AxeForging/yoink/actions"
	"github.com/AxeForging/yoink/helpers"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func socketPath() string {
	if p := os.Getenv("YOINK_SOCK"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".yoink", "yoink.sock")
}

func main() {
	helpers.SetupLogger("info")

	sockPath := socketPath()
	client := services.NewClientService(sockPath)

	runAction := actions.NewRunAction(client)
	lsAction := actions.NewLsAction(client)
	logAction := actions.NewLogAction(client)
	sendAction := actions.NewSendAction(client, false)
	sendRedactedAction := actions.NewSendAction(client, true)
	snapshotAction := actions.NewSnapshotAction(client)
	attachAction := actions.NewAttachAction(client)
	waitAction := actions.NewWaitAction(client)
	killAction := actions.NewKillAction(client)
	cleanAction := actions.NewCleanAction(client)
	daemonAction := actions.NewDaemonAction(sockPath)
	versionAction := actions.NewVersionAction(Version, BuildTime, GitCommit)

	app := cli.NewApp()
	app.Name = "yoink"
	app.Usage = "Background process manager with interactive I/O"
	app.Version = Version

	app.Flags = []cli.Flag{
		verboseFlag,
	}

	app.Before = func(c *cli.Context) error {
		if c.Bool("verbose") {
			helpers.SetupLogger("debug")
		}
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:    "run",
			Aliases: []string{"r"},
			Usage:   "Spawn a command in the background",
			Flags:   []cli.Flag{aliasFlag},
			Action:  runAction.Execute,
		},
		{
			Name:    "ls",
			Aliases: []string{"list", "ps"},
			Usage:   "List all managed processes",
			Action:  lsAction.Execute,
		},
		{
			Name:    "log",
			Aliases: []string{"l"},
			Usage:   "Show recent output from a process",
			Flags:   []cli.Flag{linesFlag, newFlag},
			Action:  logAction.Execute,
		},
		{
			Name:      "wait",
			Aliases:   []string{"w"},
			ArgsUsage: "<id|alias>...",
			Usage:     "Wait for one or more processes to finish, streaming output (exit 124 on timeout)",
			Flags:   []cli.Flag{timeoutFlag, pollFlag, messageFlag},
			Action:  waitAction.Execute,
		},
		{
			Name:    "send",
			Aliases: []string{"s"},
			Usage:   "Send input text to a running process",
			Flags:   []cli.Flag{typeModeFlag, typeDelayFlag, noEnterFlag},
			Action:  sendAction.Execute,
		},
		{
			Name:  "send-redacted",
			Usage: "Send input to a process (redacted in logs)",
			Flags: []cli.Flag{typeModeFlag, typeDelayFlag, noEnterFlag},
			Action: sendRedactedAction.Execute,
		},
		{
			Name:    "snapshot",
			Aliases: []string{"snap"},
			Usage:   "Render the current VT100 screen of a process as plain text",
			Action:  snapshotAction.Execute,
		},
		{
			Name:    "attach",
			Aliases: []string{"a"},
			Usage:   "Reattach to a running process (Ctrl+] to detach)",
			Action:  attachAction.Execute,
		},
		{
			Name:    "kill",
			Aliases: []string{"k"},
			Usage:   "Terminate a managed process",
			Flags:   []cli.Flag{forceFlag},
			Action:  killAction.Execute,
		},
		{
			Name:  "clean",
			Usage: "Remove finished/failed processes from the list",
			Action: cleanAction.Execute,
		},
		{
			Name:   "daemon",
			Usage:  "Start the yoink daemon (auto-started by other commands)",
			Action: daemonAction.Execute,
			Hidden: true,
		},
		{
			Name:   "version",
			Usage:  "Show version information",
			Action: versionAction.Execute,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
