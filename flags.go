package main

import "github.com/urfave/cli"

var verboseFlag = cli.BoolFlag{
	Name:  "verbose",
	Usage: "Enable verbose logging",
}

var linesFlag = cli.IntFlag{
	Name:  "lines, n",
	Value: 50,
	Usage: "Number of output lines to show",
}

var newFlag = cli.BoolFlag{
	Name:  "new",
	Usage: "Show only lines not yet seen by a previous --new call",
}

var forceFlag = cli.BoolFlag{
	Name:  "force, f",
	Usage: "Send SIGKILL instead of SIGTERM",
}

var aliasFlag = cli.StringFlag{
	Name:  "alias",
	Usage: "Assign an alias to the process for easier reference",
}
