package actions

import (
	"fmt"

	"github.com/urfave/cli"
)

// VersionAction prints version information.
type VersionAction struct {
	version   string
	buildTime string
	gitCommit string
}

// NewVersionAction creates a VersionAction.
func NewVersionAction(version, buildTime, gitCommit string) *VersionAction {
	return &VersionAction{
		version:   version,
		buildTime: buildTime,
		gitCommit: gitCommit,
	}
}

// Execute prints version details.
func (a *VersionAction) Execute(c *cli.Context) error {
	fmt.Printf("yoink version %s\n", a.version)
	fmt.Printf("Build time: %s\n", a.buildTime)
	fmt.Printf("Git commit: %s\n", a.gitCommit)
	return nil
}
