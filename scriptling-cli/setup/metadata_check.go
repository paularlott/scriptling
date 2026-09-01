package setup

import (
	"errors"
	"fmt"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/build"
	"github.com/paularlott/scriptling/metadata"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

// CheckScriptMetadata verifies the inline metadata block in source against
// the environment p actually provides: its registered libraries, the
// built-in CLI libraries, modules resolvable through loader, and the plugins
// loaded in manager. Source without a block passes unchanged. The CLI's
// one-shot runs, its server setup scripts, and package main entries all
// check through here, so the verdict cannot drift between modes.
//
// The check belongs after the interpreter is fully wired and before the
// source executes — callers must not register libraries or load plugins
// after it.
func CheckScriptMetadata(p *scriptling.Scriptling, loader *pack.Loader, manager *scriptlingplugin.Manager, source []byte) error {
	m, ok, err := metadata.Parse(source)
	if err != nil {
		return fmt.Errorf("script metadata: %w", err)
	}
	if !ok {
		return nil
	}

	builtins := make(map[string]bool)
	for _, name := range AllLibraryNames() {
		builtins[name] = true
	}
	resolves := func(name string) bool {
		if p.HasLibrary(name) || builtins[name] {
			return true
		}
		if loader != nil {
			if _, found, loadErr := loader.Load(name); loadErr == nil && found {
				return true
			}
		}
		return false
	}
	pluginVersion := func(name string) (string, bool) {
		if manager == nil {
			return "", false
		}
		for _, md := range manager.List() {
			if md.Name == name {
				return md.Version, true
			}
		}
		return "", false
	}

	err = m.Verify(metadata.Env{
		HostVersion:   build.Version,
		Resolves:      resolves,
		PluginVersion: pluginVersion,
	})
	var check *metadata.CheckError
	if errors.As(err, &check) && check.Has(metadata.FailurePluginMissing) {
		// The failure messages name the plugin; this says how to load one.
		return fmt.Errorf("%w\nload plugins with --plugin <path>, --plugin-dir, or SCRIPTLING_PLUGIN_DIR", err)
	}
	return err
}

// CheckMainEntryMetadata verifies the metadata block of a package main
// entry: a script entry's bytes directly, a module entry through the module
// source the loader would import. Entries whose code carries no block run as
// before. A module that cannot be loaded is not an error here — the import
// itself reports that failure with the module named.
func CheckMainEntryMetadata(p *scriptling.Scriptling, loader *pack.Loader, manager *scriptlingplugin.Manager, entry pack.MainEntry) error {
	if entry.Script != nil {
		return CheckScriptMetadata(p, loader, manager, entry.Script)
	}
	if loader == nil || entry.Module == "" {
		return nil
	}
	source, found, err := loader.Load(entry.Module)
	if err != nil || !found {
		return nil
	}
	return CheckScriptMetadata(p, loader, manager, []byte(source))
}
