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

var timeoutFlag = cli.IntFlag{
	Name:  "timeout, t",
	Value: 0,
	Usage: "Maximum seconds to wait (0 = wait forever)",
}

var pollFlag = cli.IntFlag{
	Name:  "poll, p",
	Value: 2,
	Usage: "Seconds between state checks",
}

var messageFlag = cli.StringFlag{
	Name:  "message, m",
	Usage: "Custom error message on timeout",
}

var typeModeFlag = cli.BoolFlag{
	Name:  "type",
	Usage: "Send input character by character (simulates typing)",
}

var typeDelayFlag = cli.IntFlag{
	Name:  "delay, d",
	Value: 50,
	Usage: "Milliseconds between characters when using --type",
}

var noEnterFlag = cli.BoolFlag{
	Name:  "no-enter",
	Usage: "Do not send a newline after the input",
}

var submitFlag = cli.BoolFlag{
	Name:  "submit",
	Usage: "Submit text with carriage-return Enter (CR), useful for TUIs like Claude",
}

var keyFlag = cli.StringFlag{
	Name:  "key, k",
	Usage: "Send a named key sequence instead of text (e.g. up, down, ctrl+c, f1, pageup)",
}

var cwdFlag = cli.StringFlag{
	Name:  "cwd",
	Usage: "Working directory for the process (default: your current directory)",
}

var logFileFlag = cli.StringFlag{
	Name:  "log-file",
	Usage: "Tee full raw output to this file, unbounded by the in-memory log buffer",
}

var fullFlag = cli.BoolFlag{
	Name:  "full",
	Usage: "Show the untruncated command and working directory",
}

var allFlag = cli.BoolFlag{
	Name:  "all, a",
	Usage: "Also stop and remove running processes",
}
