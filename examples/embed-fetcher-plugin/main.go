// embed-fetcher-plugin shows a host application embedding scriptling with
// fetcher plugins enabled: scripts and libraries are pulled from a plugin over
// the plugin protocol, on demand, with nothing on the local filesystem.
//
// This is the same wiring the CLI performs, in the order it matters:
//
//  1. build a plugin.Manager and load the plugins;
//  2. bridge the manager's fetchers into a pack scheme registry;
//  3. open the packages the plugins declare (or any scheme:// source);
//  4. put those bundles behind the interpreter's library loader;
//  5. fetch and run a script that lives in the plugin.
//
// Run it with:
//
//	go run ./examples/embed-fetcher-plugin
//
// It builds the fetcher-go example plugin into a temp dir and drives it, so it
// needs no external service.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/pluginpack"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
	"github.com/paularlott/scriptling/stdlib"
)

func main() {
	// Cancelling this context aborts in-flight plugin fetches, so Ctrl-C does
	// not wait out the protocol timeout.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	pluginPath, cleanup, err := buildExamplePlugin()
	if err != nil {
		return err
	}
	defer cleanup()

	// ---------------------------------------------------------------------
	// 1. Load the plugins. A host owns the manager and must close it.
	// ---------------------------------------------------------------------
	manager := plugin.NewManager(nil, func(name string, err error) {
		fmt.Fprintf(os.Stderr, "plugin %s exited: %v\n", name, err)
	})
	defer manager.Close()

	// LoadPlugin takes the executable path and its arguments separately, so a
	// path containing spaces needs no quoting.
	if _, err := manager.LoadPlugin(ctx, pluginPath, nil); err != nil {
		return fmt.Errorf("loading plugin: %w", err)
	}

	// ---------------------------------------------------------------------
	// 2. Bridge the plugin's fetcher into the pack scheme registry. This is
	//    what makes demo:// sources resolvable. Close releases the scheme,
	//    so a host can reload its plugins without restarting.
	// ---------------------------------------------------------------------
	bridge := pluginpack.New(pluginpack.Options{
		Manager: manager,
		Context: ctx,
		// Registry: pack.NewSchemeRegistry(), // use a private routing table
		// CacheDir: "/var/cache/myapp",       // override the package cache
	})
	if err := bridge.Register(); err != nil {
		return fmt.Errorf("registering fetcher schemes: %w", err)
	}
	defer bridge.Close()

	fmt.Printf("plugin schemes available: %v\n", bridge.Schemes())

	// ---------------------------------------------------------------------
	// 3. Open the plugin's library bundle — the demo://libs source every
	//    fetcher plugin attaches automatically, with the standard lib/ layout.
	// ---------------------------------------------------------------------
	bundles, err := bridge.Bundles()
	if err != nil {
		return fmt.Errorf("opening declared packages: %w", err)
	}
	for _, b := range bundles {
		fmt.Printf("attached package %s (%s)\n", b.Manifest.Name, b.Source())
	}

	// ---------------------------------------------------------------------
	// 4. Build the interpreter and put the bundles behind its loader.
	// ---------------------------------------------------------------------
	p := scriptling.New()
	stdlib.RegisterAll(p)

	// Expose the plugin's own libraries as plugin.<name> modules.
	plugin.RegisterLibraries(p, manager)

	loader := pack.NewLoader()
	for _, b := range bundles {
		if err := loader.AddBundle(b); err != nil {
			return fmt.Errorf("adding bundle: %w", err)
		}
	}
	// ApplyPackLoader keeps any existing loader ahead of the bundles, matching
	// the CLI: local files win, packages fill in the rest. Registered stdlib
	// modules are resolved before the loader either way, so a package can never
	// shadow json or os.
	bootstrap.ApplyPackLoader(p, loader)

	// ---------------------------------------------------------------------
	// 5a. Import a module that only exists inside the plugin.
	// ---------------------------------------------------------------------
	result, err := p.EvalWithContext(ctx, "import greet\ngreet.greeting('embedded host')")
	if err != nil {
		return fmt.Errorf("importing a plugin-served module: %w", err)
	}
	fmt.Printf("greet.greeting() -> %s\n", result.Inspect())

	// ---------------------------------------------------------------------
	// 5b. Fetch and run a script that lives in the plugin. Scripts are always
	//     refetched, and arrive as source text — nothing is staged to disk.
	// ---------------------------------------------------------------------
	source, err := bridge.FetchScript(ctx, "demo://scripts/hello")
	if err != nil {
		return fmt.Errorf("fetching script: %w", err)
	}
	runner := scriptling.New()
	stdlib.RegisterAll(runner)
	// The fetched script reads sys.argv, so register sys with the argv the host
	// wants the script to see.
	setup.RegisterSys(runner, []string{"demo://scripts/hello", "embedded host"})
	bootstrap.ApplyPackLoader(runner, loader)
	runner.SetSourceFile("demo://scripts/hello")
	if _, err := runner.EvalWithContext(ctx, string(source)); err != nil {
		return fmt.Errorf("running fetched script: %w", err)
	}

	// A source whose scheme no loaded plugin serves reports exactly that,
	// rather than a missing-file error.
	if _, err := pack.FetchBundle("nowhere://libs", false, ""); err != nil {
		fmt.Printf("unknown scheme reports: %v\n", err)
	}

	return nil
}

// buildExamplePlugin compiles examples/plugins/fetcher-go into a temp dir and
// returns its path. A real host would point at an installed executable.
func buildExamplePlugin() (string, func(), error) {
	dir, err := os.MkdirTemp("", "scriptling-embed-example-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	out := filepath.Join(dir, "fetcher-go")
	cmd := exec.Command("go", "build", "-o", out, "./examples/plugins/fetcher-go")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("building the example plugin (run from the repository root): %w", err)
	}
	return out, cleanup, nil
}
