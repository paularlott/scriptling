package setup

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

func TestCheckScriptMetadata(t *testing.T) {
	p := scriptling.New()
	if err := p.RegisterScriptLibrary("hostutils", "def greeting(name):\n    return name\n"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		source  string
		wantErr string
	}{
		{"no block passes", "print(1)\n", ""},
		{"builtin dependency resolves on a bare interpreter", "# /// script\n# dependencies = [\"json\"]\n# ///\nprint(1)", ""},
		{"registered script library resolves", "# /// script\n# dependencies = [\"hostutils\"]\n# ///\nprint(1)", ""},
		{
			"missing plugin is named with the load hint",
			"# /// script\n# dependencies = [\"knot.space via knot >= 1.2\"]\n# ///\nprint(1)",
			`required plugin "knot" is not loaded`,
		},
		{
			"malformed block is a hard error",
			"# /// script\n# frobnicate = true\n# ///\nprint(1)",
			`unknown key "frobnicate"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckScriptMetadata(p, nil, nil, []byte(tc.source))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}

	// The plugin hint rides along on plugin-missing failures.
	err := CheckScriptMetadata(p, nil, nil, []byte("# /// script\n# plugins = [\"knot\"]\n# ///\nprint(1)"))
	if err == nil || !strings.Contains(err.Error(), "--plugin-dir") {
		t.Fatalf("expected the plugin load hint, got: %v", err)
	}
}

func TestCheckScriptMetadataLoaderResolution(t *testing.T) {
	p := scriptling.New()
	loader := pack.NewLoader()
	loader.SetFallback(libloader.NewMemoryLoader(map[string]string{
		"bundled": "# a module provided by a package, no block\nx = 1\n",
	}))

	// A dependency satisfied by loader-resolved module source passes.
	err := CheckScriptMetadata(p, loader, nil, []byte("# /// script\n# dependencies = [\"bundled\"]\n# ///\nprint(1)"))
	if err != nil {
		t.Fatalf("expected the loader-resolved module to satisfy the dependency: %v", err)
	}
}

func TestCheckMainEntryMetadata(t *testing.T) {
	p := scriptling.New()
	loader := pack.NewLoader()
	loader.SetFallback(libloader.NewMemoryLoader(map[string]string{
		"entrymod": "# /// script\n# plugins = [\"knot >= 1.2.3\"]\n# ///\ndef main():\n    pass\n",
		"plainmod": "def main():\n    pass\n",
	}))

	// A script entry's block is checked from its bytes.
	err := CheckMainEntryMetadata(p, loader, nil, pack.MainEntry{
		Script:     []byte("# /// script\n# plugins = [\"knot\"]\n# ///\nprint(1)"),
		ScriptName: "main.py",
	})
	if err == nil || !strings.Contains(err.Error(), `required plugin "knot" is not loaded`) {
		t.Fatalf("expected the script entry's block to be checked, got: %v", err)
	}
	err = CheckMainEntryMetadata(p, loader, nil, pack.MainEntry{
		Script:     []byte("print(1)"),
		ScriptName: "main.py",
	})
	if err != nil {
		t.Fatalf("a script entry without a block passes: %v", err)
	}

	// A module entry is checked through the module source the loader imports.
	err = CheckMainEntryMetadata(p, loader, nil, pack.MainEntry{Module: "entrymod", Function: "main"})
	if err == nil || !strings.Contains(err.Error(), `plugin "knot" is not loaded`) {
		t.Fatalf("expected the module entry's block to be checked, got: %v", err)
	}
	err = CheckMainEntryMetadata(p, loader, nil, pack.MainEntry{Module: "plainmod", Function: "main"})
	if err != nil {
		t.Fatalf("a module entry without a block passes: %v", err)
	}

	// A module the loader cannot produce is left to the import error.
	if err := CheckMainEntryMetadata(p, loader, nil, pack.MainEntry{Module: "nowhere", Function: "main"}); err != nil {
		t.Fatalf("an unresolvable module entry should defer to the import error, got: %v", err)
	}
	// A module entry with no loader at all defers as well.
	if err := CheckMainEntryMetadata(p, nil, nil, pack.MainEntry{Module: "entrymod", Function: "main"}); err != nil {
		t.Fatalf("expected nil error with a nil loader, got: %v", err)
	}
}
