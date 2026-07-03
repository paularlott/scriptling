package server

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/paularlott/logger"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// mcpStdioTestServer builds a Server with the MCP exec tool enabled and returns
// the MCP server it produces, ready to serve over a stream.
func mcpStdioTestServer(t *testing.T) *mcp_lib.Server {
	t.Helper()
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	s := &Server{
		config: ServerConfig{
			MCPExecTool: true,
			LibDirs:     []string{libDir},
		},
	}
	mcpServer, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	return mcpServer
}

// TestMCPServeStreamExecTool verifies scriptling's MCP server works over a
// newline-delimited JSON-RPC stream (the stdio transport), driving it with the
// mcp library's own stream client.
func TestMCPServeStreamExecTool(t *testing.T) {
	mcpServer := mcpStdioTestServer(t)

	clientReader, serverWriter := io.Pipe() // server -> client
	serverReader, clientWriter := io.Pipe() // client -> server

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = mcpServer.ServeStream(context.Background(), serverReader, serverWriter)
	}()

	client := mcp_lib.NewStreamClient(clientReader, clientWriter, "")
	defer func() {
		client.Close()
		clientWriter.Close()
		<-serveDone
		serverWriter.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// tools/list must include the exec tool.
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := false
	for _, tool := range tools {
		if tool.Name == "execute_script" {
			found = true
		}
	}
	if !found {
		t.Fatalf("execute_script not in tools: %+v", tools)
	}

	// tools/call runs Scriptling code and returns its captured output.
	resp, err := client.CallTool(ctx, "execute_script", map[string]any{"code": "print(6 * 7)"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "42" {
		t.Fatalf("unexpected content: %+v", resp.Content)
	}
}
