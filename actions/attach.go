package actions

import (
	"fmt"
	"io"
	"os"

	"github.com/AxeForging/yoink/domain"
	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
	"golang.org/x/term"
)

// AttachAction reattaches to a running process with full PTY I/O.
type AttachAction struct {
	client *services.ClientService
}

// NewAttachAction creates an AttachAction.
func NewAttachAction(client *services.ClientService) *AttachAction {
	return &AttachAction{client: client}
}

// Execute attaches the terminal to the process. Ctrl+] to detach.
func (a *AttachAction) Execute(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: yoink attach <id|alias>")
	}

	id, alias := parseProcessRef(c.Args().First())

	if err := a.client.EnsureDaemon(); err != nil {
		return err
	}

	conn, err := a.client.Connect()
	if err != nil {
		return err
	}
	defer conn.Close()

	req := domain.DaemonRequest{Type: domain.ReqAttach, ID: id, Alias: alias}
	if err := a.client.SendRequest(conn, req); err != nil {
		return err
	}

	resp, err := a.client.ReadResponse(conn)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	stdinFd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(stdinFd)
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer term.Restore(stdinFd, oldState) //nolint:errcheck

	fmt.Fprintf(os.Stderr, "\r\nAttached to process %d (Ctrl+] to detach)\r\n", id)

	done := make(chan struct{})

	// conn -> stdout
	go func() {
		io.Copy(os.Stdout, conn) //nolint:errcheck
		close(done)
	}()

	// stdin -> conn (with Ctrl+] escape)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				for i := 0; i < n; i++ {
					if buf[i] == 0x1d { // Ctrl+]
						conn.Close()
						return
					}
				}
				conn.Write(buf[:n]) //nolint:errcheck
			}
			if err != nil {
				conn.Close()
				return
			}
		}
	}()

	<-done
	fmt.Fprintf(os.Stderr, "\r\nDetached from process %d\r\n", id)
	return nil
}
