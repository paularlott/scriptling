package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/paularlott/cli"
	"github.com/paularlott/logger"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
)

var Log logger.Logger

// RunMCPServe starts the MCP server
func RunMCPServe(ctx context.Context, cmd *cli.Command) error {
	address := cmd.GetString("address")
	toolsFolder := cmd.GetString("tools")
	bearerToken := cmd.GetString("bearer-token")
	libdir := cmd.GetString("libdir")
	validate := cmd.GetBool("validate")

	// Validate mode
	if validate {
		return validateTools(toolsFolder)
	}

	// Create MCP server
	server := mcp_lib.NewServer("scriptling-cli", "1.0.0")
	server.SetInstructions("Execute Scriptling/Python tools from the tools folder.")

	// Register tools
	if err := registerTools(server, toolsFolder, libdir); err != nil {
		return fmt.Errorf("failed to register tools: %w", err)
	}

	// Create HTTP mux
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", server.HandleRequest)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Apply bearer token middleware if configured
	var handler http.Handler = mux
	if bearerToken != "" {
		handler = bearerTokenMiddleware(bearerToken, mux)
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    address,
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		Log.Info("MCP server starting", "address", address, "tools", toolsFolder)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Log.Error("Server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	Log.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	Log.Info("Server stopped")
	return nil
}

// registerTools scans and registers all tools from the tools folder
func registerTools(server *mcp_lib.Server, toolsFolder string, libdir string) error {
	tools, err := ScanToolsFolder(toolsFolder)
	if err != nil {
		return err
	}

	for toolName, meta := range tools {
		scriptPath := filepath.Join(toolsFolder, toolName+".py")

		// Build tool definition using helper
		tool := toolmetadata.BuildMCPTool(toolName, meta)

		// Register tool with handler
		handler := createToolHandler(scriptPath, libdir)
		server.RegisterTool(tool, handler)

		mode := "native"
		if meta.Discoverable {
			mode = "discoverable"
		}
		Log.Info("Registered tool", "name", toolName, "params", len(meta.Parameters), "mode", mode)
	}

	return nil
}

// createToolHandler creates a handler function for a tool
func createToolHandler(scriptPath string, libdir string) func(context.Context, *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
	return func(ctx context.Context, req *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
		// Read script
		script, err := os.ReadFile(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script: %w", err)
		}

		// Create new Scriptling instance and set up libraries
		p := scriptling.New()
		SetupScriptling(p, libdir, false) // false = don't register interact library

		// Convert request arguments to map
		params := req.Args()

		// Run tool script
		response, exitCode, err := scriptlingmcp.RunToolScript(ctx, p, string(script), params)
		if err != nil {
			return nil, fmt.Errorf("script execution failed: %w", err)
		}

		if exitCode != 0 {
			return nil, fmt.Errorf("script exited with code %d: %s", exitCode, response)
		}

		return mcp_lib.NewToolResponseText(response), nil
	}
}

// bearerTokenMiddleware creates authentication middleware
func bearerTokenMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + token

		if auth != expected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateTools validates all tools in the folder
func validateTools(toolsFolder string) error {
	tools, err := ScanToolsFolder(toolsFolder)
	if err != nil {
		return err
	}

	hasErrors := false
	for toolName, meta := range tools {
		scriptPath := filepath.Join(toolsFolder, toolName+".py")
		warnings := ValidateTool(toolName, meta, scriptPath)

		if len(warnings) > 0 {
			hasErrors = true
			fmt.Printf("Tool '%s':\n", toolName)
			for _, warning := range warnings {
				fmt.Printf("  - %s\n", warning)
			}
		}
	}

	if !hasErrors {
		fmt.Printf("All %d tools validated successfully\n", len(tools))
	}

	return nil
}
