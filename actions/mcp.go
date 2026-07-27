package actions

import (
	"context"

	"github.com/AxeForging/yoink/services"
	"github.com/urfave/cli"
)

// MCPAction serves the yoink daemon over the Model Context Protocol on stdio,
// so agents can manage background processes as native tools.
type MCPAction struct {
	client  *services.ClientService
	version string
}

// NewMCPAction creates an MCPAction.
func NewMCPAction(client *services.ClientService, version string) *MCPAction {
	return &MCPAction{client: client, version: version}
}

// Execute serves MCP on stdio until the client disconnects.
//
// Nothing here may write to stdout: it is the protocol stream.
func (a *MCPAction) Execute(_ *cli.Context) error {
	return services.NewMCPServer(a.client, a.version).Serve(context.Background())
}
