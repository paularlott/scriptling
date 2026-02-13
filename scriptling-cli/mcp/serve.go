package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/paularlott/cli"
	"github.com/paularlott/logger"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
)

var Log logger.Logger

// Script execution mode constants
const (
	ScriptExecuteOff  = "off"
	ScriptExecuteSafe = "safe"
	ScriptExecuteFull = "full"
)

// reloadableHandler wraps an MCP server pointer to allow hot-reloading of tools
type reloadableHandler struct {
	server atomic.Pointer[mcp_lib.Server]
}

func (h *reloadableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server := h.server.Load()
	if server == nil {
		http.Error(w, "Server not ready", http.StatusServiceUnavailable)
		return
	}
	(*server).HandleRequest(w, r)
}

// RunMCPServe starts the MCP server
func RunMCPServe(ctx context.Context, cmd *cli.Command) error {
	address := cmd.GetString("address")
	toolsFolder := cmd.GetString("tools")
	bearerToken := cmd.GetString("bearer-token")
	libdir := cmd.GetString("libdir")
	validate := cmd.GetBool("validate")
	scriptExecute := cmd.GetString("allow-script-execute")

	// Validate mode
	if validate {
		return validateTools(toolsFolder)
	}

	// Determine safe mode based on script execution flag
	safeMode := false
	if scriptExecute == ScriptExecuteSafe {
		safeMode = true
	}

	// Create reloadable handler for hot-reloading tools
	reloadHandler := &reloadableHandler{}

	// Create initial server
	server, err := createServer(toolsFolder, libdir, scriptExecute, safeMode)
	if err != nil {
		return err
	}
	reloadHandler.server.Store(server)

	// Create HTTP mux
	mux := http.NewServeMux()
	mux.Handle("/mcp", reloadHandler)
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
		Log.Info("MCP server starting", "address", address, "tools", toolsFolder, "script-execute", scriptExecute)
		Log.Info("Press Ctrl+C to exit, tools auto-reload on file changes, SIGHUP to force reload")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Log.Error("Server error", "error", err)
		}
	}()

	// Set up file watcher for tools folder
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Log.Warn("Failed to create file watcher, auto-reload disabled", "error", err)
	} else {
		if err := watcher.Add(toolsFolder); err != nil {
			Log.Warn("Failed to watch tools folder, auto-reload disabled", "error", err)
			watcher.Close()
			watcher = nil
		} else {
			Log.Info("Watching tools folder for changes", "path", toolsFolder)
		}
	}

	// Wait for signals or file changes
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1)

	// Debounce timer for file changes
	var reloadTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				continue
			}
			// Only watch for .toml file changes (create, write, remove, rename)
			if filepath.Ext(event.Name) == ".toml" {
				// Debounce: reset timer on each event
				if reloadTimer != nil {
					reloadTimer.Stop()
				}
				reloadTimer = time.AfterFunc(debounceDuration, func() {
					Log.Debug("Tool file changed", "event", event.Op.String(), "file", filepath.Base(event.Name))
					doReload(toolsFolder, libdir, scriptExecute, safeMode, reloadHandler)
				})
			}
		case err, ok := <-watcher.Errors:
			if ok {
				Log.Error("File watcher error", "error", err)
			}
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP, syscall.SIGUSR1:
				doReload(toolsFolder, libdir, scriptExecute, safeMode, reloadHandler)
			default:
				// SIGINT or SIGTERM - shutdown
				Log.Info("Shutting down server...")

				if watcher != nil {
					watcher.Close()
				}
				if reloadTimer != nil {
					reloadTimer.Stop()
				}

				// Graceful shutdown
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := httpServer.Shutdown(shutdownCtx); err != nil {
					return fmt.Errorf("server shutdown failed: %w", err)
				}

				Log.Info("Server stopped")
				return nil
			}
		}
	}
}

// doReload reloads all tools
func doReload(toolsFolder, libdir, scriptExecute string, safeMode bool, reloadHandler *reloadableHandler) {
	Log.Info("Reloading tools...")
	newServer, err := createServer(toolsFolder, libdir, scriptExecute, safeMode)
	if err != nil {
		Log.Error("Failed to reload tools", "error", err)
	} else {
		reloadHandler.server.Store(newServer)
		Log.Info("Tools reloaded successfully")
	}
}

// createServer creates a new MCP server with all tools registered
func createServer(toolsFolder, libdir, scriptExecute string, safeMode bool) (*mcp_lib.Server, error) {
	server := mcp_lib.NewServer("scriptling-cli", "1.0.0")
	server.SetInstructions("Execute Scriptling/Python tools from the tools folder.")

	// Register tools from folder
	if err := registerTools(server, toolsFolder, libdir, safeMode); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	// Register execute_scriptling tool if requested
	if scriptExecute != "" && scriptExecute != ScriptExecuteOff {
		if err := registerScriptExecutionTool(server, libdir, safeMode); err != nil {
			return nil, fmt.Errorf("failed to register script execution tool: %w", err)
		}
	}

	return server, nil
}

// registerTools scans and registers all tools from the tools folder
func registerTools(server *mcp_lib.Server, toolsFolder string, libdir string, safeMode bool) error {
	tools, err := ScanToolsFolder(toolsFolder)
	if err != nil {
		return err
	}

	for toolName, meta := range tools {
		scriptPath := filepath.Join(toolsFolder, toolName+".py")

		// Build tool definition using helper
		tool := toolmetadata.BuildMCPTool(toolName, meta)

		// Register tool with handler
		handler := createToolHandler(scriptPath, libdir, safeMode)
		server.RegisterTool(tool, handler)

		mode := "native"
		if meta.Discoverable {
			mode = "discoverable"
		}
		Log.Info("Registered tool", "name", toolName, "params", len(meta.Parameters), "mode", mode)
	}

	return nil
}

// registerScriptExecutionTool registers the execute_scriptling tool
func registerScriptExecutionTool(server *mcp_lib.Server, libdir string, safeMode bool) error {
	var description string
	if safeMode {
		description = "Execute a Scriptling script in sandboxed mode (no file/network/subprocess access). Use `help()` in your script to discover available modules and functions."
	} else {
		description = "Execute a Scriptling script (Python 3-like syntax) and return the result. Use `help()` in your script to discover available modules and functions."
	}

	tool := mcp_lib.NewTool("execute_scriptling", description,
		mcp_lib.String("code", "The Scriptling/Python code to execute", mcp_lib.Required()),
	)

	handler := createScriptExecutionHandler(libdir, safeMode)
	server.RegisterTool(tool, handler)

	mode := "safe"
	if !safeMode {
		mode = "full"
	}
	Log.Info("Registered script execution tool", "mode", mode)

	return nil
}

// createScriptExecutionHandler creates a handler for the execute_scriptling tool
func createScriptExecutionHandler(libdir string, safeMode bool) func(context.Context, *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
	return func(ctx context.Context, req *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
		// Get code parameter
		code, ok := req.Args()["code"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'code' parameter")
		}

		// Create new Scriptling instance and set up libraries
		p := scriptling.New()
		SetupScriptling(p, libdir, false, safeMode)

		// Execute script with timeout from context
		result, err := p.EvalWithContext(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("script execution failed: %w", err)
		}

		// Convert result to string response
		var response string
		if result != nil {
			response = result.Inspect()
		}

		return mcp_lib.NewToolResponseText(response), nil
	}
}

// createToolHandler creates a handler function for a tool
func createToolHandler(scriptPath string, libdir string, safeMode bool) func(context.Context, *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
	return func(ctx context.Context, req *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
		// Read script
		script, err := os.ReadFile(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script: %w", err)
		}

		// Create new Scriptling instance and set up libraries
		p := scriptling.New()
		SetupScriptling(p, libdir, false, safeMode) // false = don't register interact library

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
