package console_test

import (
	"testing"

	"github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/cli/tui"
)

func TestPanelAccessFromBackgroundTasks(t *testing.T) {
	console.ResetConsole()

	t.Run("Background tasks can create and write to panels", func(t *testing.T) {
		sharedTUI := console.TUI()

		logPanel := sharedTUI.CreatePanel(tui.PanelConfig{
			Name:       "logs",
			Width:      -25,
			MinWidth:   15,
			Scrollable: true,
			Title:      "Logs",
		})

		if logPanel == nil {
			t.Fatal("Expected CreatePanel to return a valid panel")
		}

		logPanel.WriteString("Background task: Starting up...\n")
		logPanel.WriteString("Background task: Processing data...\n")

		retrievedPanel := sharedTUI.Panel("logs")
		if retrievedPanel == nil {
			t.Fatal("Expected Panel() to return the created panel")
		}
		if retrievedPanel != logPanel {
			t.Fatal("Expected Panel() to return the same panel instance")
		}
	})

	t.Run("Module-level panel() accesses shared panels", func(t *testing.T) {
		console.ResetConsole()

		sharedTUI := console.TUI()
		sharedTUI.CreatePanel(tui.PanelConfig{
			Name:  "shared-panel",
			Title: "Shared Panel",
		})
		sharedTUI.Panel("shared-panel").WriteString("Message from TUI\n")

		p := newInterpreter(t)
		_, err := p.Eval(`
import scriptling.console as console

panel = console.panel("shared-panel")
assert panel is not None
assert hasattr(panel, "write")
assert hasattr(panel, "set_title")

panel.write("Message from script\n")
`)
		if err != nil {
			t.Fatalf("Script execution failed: %v", err)
		}
	})
}
