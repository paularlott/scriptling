package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/paularlott/cli"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/lint"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// runMetadataCheck drives the real command tree with real argv, stubbing
// execution so the Run body is exactly checkScriptMetadata on a fresh
// interpreter — the same call runScriptling makes after setup.
func runMetadataCheck(t *testing.T, args []string) (error, bool) {
	t.Helper()
	previousArgs := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = previousArgs })

	root := buildRootCommand()
	var checkErr error
	ran := false
	forEachCommand(root, func(c *cli.Command) {
		c.Run = func(ctx context.Context, cmd *cli.Command) error {
			ran = true
			checkErr = checkScriptMetadata(ctx, cmd, scriptling.New(), nil)
			return checkErr
		}
	})
	root.PreRun = func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
		return ctx, nil
	}
	root.PostRun = nil

	_ = root.Execute(context.Background())
	return checkErr, ran
}

func TestCheckScriptMetadata(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "code without a block is untouched",
			args: []string{"scriptling", "-c", "print(1)"},
		},
		{
			name: "code with satisfiable requirements passes",
			args: []string{"scriptling", "-c", "# /// script\n# requires-scriptling = \">=0.1\"\n# dependencies = [\"requests\"]\n# ///\nprint(1)"},
		},
		{
			name:    "missing plugin is named with the load hint",
			args:    []string{"scriptling", "-c", "# /// script\n# plugins = [\"knot >= 1.2.3\"]\n# ///\nprint(1)"},
			wantErr: "required plugin \"knot\" is not loaded",
		},
		{
			name:    "via without a constraint still promotes a missing plugin",
			args:    []string{"scriptling", "-c", "# /// script\n# dependencies = [\"scriptling.sql via sql\"]\n# ///\nprint(1)"},
			wantErr: "required plugin \"sql\" is not loaded",
		},
		{
			name:    "unresolved library without a provider is named",
			args:    []string{"scriptling", "-c", "# /// script\n# dependencies = [\"definitely.not.here\"]\n# ///\nprint(1)"},
			wantErr: "required library \"definitely.not.here\" is not available",
		},
		{
			name:    "malformed block is a hard error",
			args:    []string{"scriptling", "-c", "# /// script\n# frobnicate = true\n# ///\nprint(1)"},
			wantErr: "unknown key \"frobnicate\"",
		},
		{
			name: "no code and no file has no metadata contract",
			args: []string{"scriptling"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err, ran := runMetadataCheck(t, tc.args)
			if !ran {
				t.Fatalf("Run never executed for %v", tc.args)
			}
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestCheckScriptMetadataFileSource(t *testing.T) {
	dir := t.TempDir()
	failing := filepath.Join(dir, "failing.py")
	if err := os.WriteFile(failing, []byte("# /// script\n# plugins = [\"knot\"]\n# ///\nprint(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	passing := filepath.Join(dir, "passing.py")
	if err := os.WriteFile(passing, []byte("# /// script\n# dependencies = [\"requests\"]\n# ///\nprint(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err, ran := runMetadataCheck(t, []string{"scriptling", failing})
	if !ran {
		t.Fatal("Run never executed")
	}
	if err == nil || !strings.Contains(err.Error(), `required plugin "knot" is not loaded`) {
		t.Fatalf("expected the missing plugin error from a file source, got: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "--plugin-dir") {
		t.Fatalf("expected the plugin load hint, got: %v", err)
	}

	err, ran = runMetadataCheck(t, []string{"scriptling", passing})
	if !ran {
		t.Fatal("Run never executed")
	}
	if err != nil {
		t.Fatalf("expected a satisfiable file to pass, got: %v", err)
	}
}

// TestCheckScriptMetadataLoadedPlugin runs the whole check against a real
// plugin process: a shell helper declaring the bare name "metadata-demo" at
// version 1.0.0. The bare requirement must find it under its registered
// plugin.metadata-demo name, and its version must be constraint-checked.
func TestCheckScriptMetadataLoadedPlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell plugin helper is unix-only")
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "metadata-demo")
	script := `#!/bin/sh
read req
echo '{"jsonrpc":"2.0","id":1,"result":{"protocol":"1.0","transport":"json","library":{"name":"metadata-demo","version":"1.0.0","description":"metadata test"},"capabilities":[],"schema":{"functions":[],"classes":[],"constants":[]}}}'
read req
`
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	manager, err := loadPluginManager(context.Background(), nil, []string{helper}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("loadPluginManager: %v", err)
	}
	defer manager.Close()

	previousManager := pluginManager
	pluginManager = manager
	t.Cleanup(func() { pluginManager = previousManager })

	satisfiable := "# /// script\n# plugins = [\"metadata-demo >= 1.0\"]\n# ///\nprint(1)"
	if err, _ := runMetadataCheck(t, []string{"scriptling", "-c", satisfiable}); err != nil {
		t.Errorf("expected the loaded plugin to satisfy the requirement: %v", err)
	}

	// No constraint at all: loaded is enough, at any version.
	bare := "# /// script\n# plugins = [\"metadata-demo\"]\n# ///\nprint(1)"
	if err, _ := runMetadataCheck(t, []string{"scriptling", "-c", bare}); err != nil {
		t.Errorf("expected a constraintless plugin requirement to pass when loaded: %v", err)
	}

	// A via entry with no constraint, its library unresolvable in this test
	// build, promotes the plugin — and the loaded plugin satisfies it.
	viaBare := "# /// script\n# dependencies = [\"acme.widgets via metadata-demo\"]\n# ///\nprint(1)"
	if err, _ := runMetadataCheck(t, []string{"scriptling", "-c", viaBare}); err != nil {
		t.Errorf("expected the loaded plugin to satisfy the promoted constraintless requirement: %v", err)
	}

	tooOld := "# /// script\n# plugins = [\"metadata-demo >= 2.0\"]\n# ///\nprint(1)"
	err, _ = runMetadataCheck(t, []string{"scriptling", "-c", tooOld})
	if err == nil || !strings.Contains(err.Error(), `plugin "metadata-demo" is version 1.0.0, but this script needs >=2.0`) {
		t.Errorf("expected a version failure naming both versions, got: %v", err)
	}
}

// TestResolveMainEntryMetadata drives the package main entry path with a
// real bundle: the entry's block is resolved from the bundle the same way
// runScriptling does, and an unmet requirement names the plugin.
func TestResolveMainEntryMetadata(t *testing.T) {
	writeBundle := func(mainScript string) string {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "manifest.toml"), []byte("name = \"metapk\"\nversion = \"1.0.0\"\nmain = \"main.py\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte(mainScript), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	// A failing block in the entry script refuses the run. A main-only
	// bundle is a library bundle (only serve makes an app bundle), and
	// ResolveMain is exactly the no-file no-code path through its main.
	dir := writeBundle("# /// script\n# plugins = [\"knot >= 1.2.3\"]\n# ///\nprint(1)\n")
	_, libs, err := openBundles([]string{dir}, false, t.TempDir())
	if err != nil {
		t.Fatalf("openBundles: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected one library bundle, got %d", len(libs))
	}
	loader := pack.NewLoader()
	if err := loader.AddBundle(libs[0]); err != nil {
		t.Fatalf("AddBundle: %v", err)
	}
	entry, found, err := loader.ResolveMain()
	if err != nil || !found || entry.Script == nil {
		t.Fatalf("ResolveMain: found=%v entry=%+v err=%v", found, entry, err)
	}
	err = setup.CheckMainEntryMetadata(scriptling.New(), loader, nil, entry)
	if err == nil || !strings.Contains(err.Error(), `plugin "knot" is not loaded`) {
		t.Fatalf("expected the entry script's block to refuse the run, got: %v", err)
	}

	// A satisfiable block in the entry script passes.
	dir = writeBundle("# /// script\n# dependencies = [\"json\"]\n# ///\nprint(1)\n")
	_, libs, err = openBundles([]string{dir}, false, t.TempDir())
	if err != nil {
		t.Fatalf("openBundles: %v", err)
	}
	loader = pack.NewLoader()
	if err := loader.AddBundle(libs[0]); err != nil {
		t.Fatalf("AddBundle: %v", err)
	}
	entry, found, err = loader.ResolveMain()
	if err != nil || !found {
		t.Fatalf("ResolveMain: found=%v err=%v", found, err)
	}
	if err := setup.CheckMainEntryMetadata(scriptling.New(), loader, nil, entry); err != nil {
		t.Fatalf("expected a satisfiable entry block to pass, got: %v", err)
	}
}

func TestLintMetadata(t *testing.T) {
	// A well-formed block adds nothing.
	result := &lint.Result{}
	lintMetadata(result, "s.py", []byte("# /// script\n# requires-scriptling = \">=0.24\"\n# ///\nprint(1)\n"))
	if result.HasErrors || len(result.Errors) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Errors)
	}

	// A malformed block is a lint error carrying file, line, and code.
	result = &lint.Result{}
	lintMetadata(result, "s.py", []byte("# /// script\n# frobnicate = true\n# ///\nprint(1)\n"))
	if !result.HasErrors || len(result.Errors) != 1 {
		t.Fatalf("expected one error, got %+v", result.Errors)
	}
	issue := result.Errors[0]
	if issue.File != "s.py" || issue.Line != 1 || issue.Severity != lint.SeverityError || issue.Code != "metadata" {
		t.Errorf("unexpected error: %+v", issue)
	}
	if !strings.Contains(issue.Message, "frobnicate") {
		t.Errorf("message %q should name the unknown key", issue.Message)
	}
}

// TestCheckScriptMetadataErrorsAsCheckError guards the hint wrapping: the
// plugin-missing hint is added, and only for plugin failures.
func TestCheckScriptMetadataErrorsAsCheckError(t *testing.T) {
	err, _ := runMetadataCheck(t, []string{"scriptling", "-c", "# /// script\n# dependencies = [\"nope.nope\"]\n# ///\nprint(1)"})
	if err == nil {
		t.Fatal("expected a library failure")
	}
	if strings.Contains(err.Error(), "--plugin-dir") {
		t.Errorf("a library-only failure should not carry the plugin hint: %v", err)
	}

	err, _ = runMetadataCheck(t, []string{"scriptling", "-c", "# /// script\n# plugins = [\"knot\"]\n# ///\nprint(1)"})
	if err == nil {
		t.Fatal("expected a plugin failure")
	}
	if !strings.Contains(err.Error(), "--plugin-dir") {
		t.Errorf("a plugin failure should carry the plugin hint: %v", err)
	}
}
