package logging_test

import (
	"bytes"
	"strings"
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/logging"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if logging.LibraryName != "logging" {
		t.Errorf("logging.LibraryName = %q, want %q (scripts import it by this name)", logging.LibraryName, "logging")
	}
}

func TestLoggingRegisterWritesToTheGivenLogger(t *testing.T) {
	var buf bytes.Buffer
	p := scriptling.New()
	logging.Register(p, logslog.New(logslog.Config{Level: "info", Format: "json", Writer: &buf}))
	scripttest.Eval(t, p, "import logging\nlogging.info('registration-check')\nlogging.warning('second')")
	out := buf.String()
	for _, want := range []string{"registration-check", "second"} {
		if !strings.Contains(out, want) {
			t.Errorf("logger output %q lacks %q", out, want)
		}
	}
}

func TestLoggingRegisterDefault(t *testing.T) {
	p := scriptling.New()
	logging.RegisterDefault(p)
	// The default logger writes to stdout; a debug message is below its
	// level, so the call is exercised without noise.
	scripttest.Eval(t, p, "import logging\nlogging.debug('quiet')")
}

func TestLoggingNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "logging", "it is only available after logging.Register(p, logger)")
}
