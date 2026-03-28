// Example demonstrating the module-level console API
// and how background tasks can access the shared TUI.
package console

import (
	"context"
	"fmt"
	"time"

	"github.com/paularlott/cli/tui"
)

// ExampleBackgroundTask demonstrates how a background goroutine can
// access the shared console TUI and write to panels.
func ExampleBackgroundTask() {
	ResetConsole()

	sharedTUI := TUI()

	logsPanel := sharedTUI.CreatePanel(tui.PanelConfig{
		Name:       "logs",
		Width:      -30,
		MinWidth:   20,
		Scrollable: true,
		Title:      "Background Logs",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		messages := []string{
			"Starting background service...",
			"Connected to database",
			"Processing data...",
			"Task completed successfully",
		}

		for i, msg := range messages {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logEntry := fmt.Sprintf("[%d] %s\n", i+1, msg)
				logsPanel.WriteString(logEntry)
			}
		}
	}()

	time.Sleep(3 * time.Second)
	cancel()

	fmt.Println("Background task example completed")
}

// ExampleModuleLevelAPI demonstrates the new module-level console API.
func ExampleModuleLevelAPI() {
	ResetConsole()

	sharedTUI := TUI()

	sharedTUI.CreatePanel(tui.PanelConfig{
		Name:  "shared-panel",
		Title: "Shared Panel",
	})

	sharedTUI.Panel("shared-panel").WriteString("Message from TUI\n")

	fmt.Println("Module-level API: panels accessible via TUI() singleton")
}
