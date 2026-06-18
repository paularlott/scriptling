package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// RunServer is the main entry point for running the server
func RunServer(ctx context.Context, config ServerConfig) error {
	Log.Debug("Starting server", "address", config.Address, "tls", config.TLSGenerate || (config.TLSCert != "" && config.TLSKey != ""), "mcp_tools", config.MCPToolsDir != "", "mcp_exec", config.MCPExecTool, "web_root", config.WebRoot)
	server, err := NewServer(config)
	if err != nil {
		return err
	}

	if err := server.Start(); err != nil {
		return err
	}

	Log.Info("Server started", "address", config.Address)
	if config.MCPToolsDir != "" {
		Log.Info(getReloadMessage())
	} else {
		Log.Info("Press Ctrl+C to exit")
	}

	sigChan := make(chan os.Signal, 1)
	signals := make([]os.Signal, 0, 2+len(reloadSignals))
	signals = append(signals, os.Interrupt, syscall.SIGTERM)
	for _, sig := range reloadSignals {
		signals = append(signals, sig)
	}
	signal.Notify(sigChan, signals...)

	var watcherDone chan struct{}
	if server.watcher != nil {
		watcherDone = make(chan struct{})
		go func() {
			defer close(watcherDone)
			for {
				select {
				case event, ok := <-server.watcher.Events:
					if !ok {
						return
					}
					ext := filepath.Ext(event.Name)
					if ext == ".toml" || ext == ".py" {
						if server.reloadDebounce != nil {
							server.reloadDebounce.Stop()
						}
						eventCopy := event
						server.reloadDebounce = time.AfterFunc(server.debounceDuration, func() {
							Log.Debug("Tool file changed", "event", eventCopy.Op.String(), "file", filepath.Base(eventCopy.Name))
							server.reloadMCPTools()
						})
					}
				case err, ok := <-server.watcher.Errors:
					if ok {
						Log.Error("File watcher error", "error", err)
					}
				}
			}
		}()
	}

	// Wait for signals
	sig := <-sigChan

	// Handle reload signals
	if sysSig, ok := sig.(syscall.Signal); ok && isReloadSignal(sysSig) {
		if config.MCPToolsDir != "" {
			server.reloadMCPTools()
		}
		// Continue waiting for shutdown signal
		sig = <-sigChan
	}

	// Clean shutdown
	Log.Info("Shutting down server...")

	if server.watcher != nil {
		server.watcher.Close()
		<-watcherDone
	}
	if server.reloadDebounce != nil {
		server.reloadDebounce.Stop()
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	Log.Info("Server stopped")
	return nil
}

// RunJSONRPCServer runs the stdio JSON-RPC 2.0 server. It performs the same
// bootstrap as RunServer (setup script registers handlers via
// runtime.jsonrpc.method/notification), then serves requests from stdin until
// stdin closes or a terminating signal arrives.
func RunJSONRPCServer(ctx context.Context, config ServerConfig) error {
	Log.Debug("Starting JSON-RPC stdio server")
	server, err := NewServer(config)
	if err != nil {
		return err
	}

	if len(server.jsonrpcMethods) == 0 && len(server.jsonrpcNotifications) == 0 {
		Log.Warn("JSON-RPC server started with no methods or notifications registered")
	}

	if err := server.RunJSONRPCStdio(ctx); err != nil {
		return fmt.Errorf("json-rpc server failed: %w", err)
	}

	Log.Info("JSON-RPC server stopped")
	return nil
}
