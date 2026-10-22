package telegram

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
)

// Local media is read only inside the allowed paths (vuln-0001). A source
// that is not an allowed local file is a Telegram file ID: one with a path
// separator cannot be an ID and is refused, any other is sent as an ID and
// never read, even when a file by that name exists outside the jail.
func TestResolveMediaHonorsAllowedPaths(t *testing.T) {
	base := t.TempDir()
	jail := filepath.Join(base, "jail")
	os.MkdirAll(jail, 0o755)
	inside := filepath.Join(jail, "in.txt")
	outside := filepath.Join(base, "secret.txt")
	os.WriteFile(inside, []byte("inside"), 0o644)
	os.WriteFile(outside, []byte("secret"), 0o644)

	c := &telegramClient{files: shared.FilePolicy([]string{jail})}
	if _, _, _, err := c.resolveMedia(outside, false); !errors.Is(err, fssecurity.ErrPathNotAllowed) {
		t.Fatalf("outside path: err = %v, want ErrPathNotAllowed", err)
	}
	if data, _, _, err := c.resolveMedia(inside, false); err != nil || string(data) != "inside" {
		t.Fatalf("inside path: %q %v", data, err)
	}

	// A bare name in a working directory outside the jail: a file ID, not read.
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	data, isURL, fileID, err := c.resolveMedia("secret.txt", false)
	if err != nil || data != nil || isURL || fileID != "secret.txt" {
		t.Fatalf("bare name: data=%q isURL=%v fileID=%q err=%v", data, isURL, fileID, err)
	}
	if _, _, fileID, err := c.resolveMedia("AgACAgIAAxkBAAIBf2Z", false); err != nil || fileID != "AgACAgIAAxkBAAIBf2Z" {
		t.Fatalf("file ID: %q %v", fileID, err)
	}
}
