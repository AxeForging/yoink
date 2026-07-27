// MCP server: exposes the yoink daemon over the Model Context Protocol (stdio)
// so agents can manage background processes as native tools instead of shelling
// out and parsing terminal output.
//
// Every tool is a thin translation into domain.DaemonRequest and back, sent
// through the same ClientService the CLI uses. The MCP server is a client of
// the daemon, not a second process manager, so an agent and a human in their
// own shell see exactly the same set of processes.
package services

import (
	"context"
	"fmt"

	"github.com/AxeForging/yoink/domain"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer serves yoink's process manager over MCP.
type MCPServer struct {
	client *ClientService
	// version is reported in the MCP initialize handshake.
	version string
}

// NewMCPServer builds an MCP server that talks to the daemon behind client.
func NewMCPServer(client *ClientService, version string) *MCPServer {
	return &MCPServer{client: client, version: version}
}

// Serve registers the tools and serves MCP over stdio until ctx is cancelled or
// the client disconnects.
//
// stdout is the protocol stream: nothing on this path may print to it.
func (s *MCPServer) Serve(ctx context.Context) error {
	return s.sdkServer().Run(ctx, &mcpsdk.StdioTransport{})
}

// sdkServer builds the underlying SDK server with all tools registered. Tests
// connect it to an in-memory transport.
func (s *MCPServer) sdkServer() *mcpsdk.Server {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "yoink",
		Version: s.version,
	}, nil)
	s.registerTools(srv)
	return srv
}

// do sends a daemon request and converts a protocol-level failure into an
// error, so handlers can deal only with successful responses.
func (s *MCPServer) do(req domain.DaemonRequest) (domain.DaemonResponse, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return domain.DaemonResponse{}, err
	}
	if !resp.Success {
		return domain.DaemonResponse{}, fmt.Errorf("%s", resp.Error)
	}
	return resp, nil
}

// resolveRef validates a process reference and splits it for the daemon.
func resolveRef(ref string) (id int, alias string, err error) {
	if ref == "" {
		return 0, "", fmt.Errorf("process is required (a numeric id or an alias)")
	}
	id, alias = ParseProcessRef(ref)
	return id, alias, nil
}
