package console_test

import (
	"testing"

	"github.com/paularlott/scriptling/extlibs/console"
)

func TestConsoleSharedTUI(t *testing.T) {
	console.ResetConsole()

	t.Run("Multiple TUI() calls return same singleton", func(t *testing.T) {
		tui1 := console.TUI()
		tui2 := console.TUI()
		if tui1 != tui2 {
			t.Fatal("Expected both TUI() calls to return the same singleton instance")
		}
	})

	t.Run("TUI returns non-nil instance", func(t *testing.T) {
		console.ResetConsole()
		sharedTUI := console.TUI()
		if sharedTUI == nil {
			t.Fatal("Expected TUI() to return non-nil TUI instance")
		}
		sameTUI := console.TUI()
		if sharedTUI != sameTUI {
			t.Fatal("Expected TUI() to return the same singleton instance")
		}
	})
}

func TestTUIAccessForBackgroundTasks(t *testing.T) {
	console.ResetConsole()
	tuiInstance := console.TUI()
	if tuiInstance == nil {
		t.Fatal("Expected TUI() to return a valid TUI instance for background tasks")
	}
}
