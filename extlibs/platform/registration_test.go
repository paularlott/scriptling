package platform_test

import (
	"runtime"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/platform"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if platform.LibraryName != "platform" {
		t.Errorf("platform.LibraryName = %q, want %q (scripts import it by this name)", platform.LibraryName, "platform")
	}
}

func TestPlatformRegisterAndQuery(t *testing.T) {
	p := scriptling.New()
	platform.Register(p)
	system := scripttest.EvalString(t, p, "import platform\nplatform.system()")
	want := map[string]string{"darwin": "Darwin", "linux": "Linux", "windows": "Windows", "freebsd": "FreeBSD"}[runtime.GOOS]
	if want != "" && system != want {
		t.Errorf("platform.system() = %q, want %q on %s", system, want, runtime.GOOS)
	}
	if system == "" {
		t.Error("platform.system() is empty")
	}
	if node := scripttest.EvalString(t, p, "import platform\nplatform.node()"); node == "" {
		t.Error("platform.node() (hostname) is empty")
	}
}

func TestPlatformNotInStandardTier(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "platform", "platform moved out of stdlib; register it with platform.Register(p)")
}
