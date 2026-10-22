package discord

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
)

// send_file reads a local file only inside the allowed paths (vuln-0001);
// URLs and base64 are not files and are unaffected.
func TestResolveMediaHonorsAllowedPaths(t *testing.T) {
	base := t.TempDir()
	jail := filepath.Join(base, "jail")
	os.MkdirAll(jail, 0o755)
	inside := filepath.Join(jail, "in.txt")
	outside := filepath.Join(base, "secret.txt")
	os.WriteFile(inside, []byte("inside"), 0o644)
	os.WriteFile(outside, []byte("secret"), 0o644)

	c := &discordClient{files: shared.FilePolicy([]string{jail})}
	if _, _, err := c.resolveMedia(outside, false); !errors.Is(err, fssecurity.ErrPathNotAllowed) {
		t.Fatalf("outside path: err = %v, want ErrPathNotAllowed", err)
	}
	data, _, err := c.resolveMedia(inside, false)
	if err != nil || string(data) != "inside" {
		t.Fatalf("inside path: %q %v", data, err)
	}
	if _, isURL, err := c.resolveMedia("https://example.com/x.png", false); err != nil || !isURL {
		t.Fatalf("URL: %v %v", isURL, err)
	}
	if data, _, err := c.resolveMedia("aGk=", true); err != nil || string(data) != "hi" {
		t.Fatalf("base64: %q %v", data, err)
	}
}
