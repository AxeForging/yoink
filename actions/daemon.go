package actions

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/AxeForging/yoink/helpers"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// DaemonAction starts the yoink daemon in the foreground.
type DaemonAction struct {
	sockPath string
}

// NewDaemonAction creates a DaemonAction.
func NewDaemonAction(sockPath string) *DaemonAction {
	return &DaemonAction{sockPath: sockPath}
}

// Execute starts the daemon and blocks until shutdown.
func (a *DaemonAction) Execute(c *cli.Context) error {
	os.MkdirAll(filepath.Dir(a.sockPath), 0o755) //nolint:errcheck

	pm := services.NewProcessManager(1000)
	server := services.NewDaemonServer(a.sockPath, pm, helpers.Log)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		helpers.Log.Info().Msg("shutting down daemon")
		pm.Shutdown()
		server.Stop()
	}()

	return server.Start()
}
