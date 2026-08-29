package plugin

import (
	"sort"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/object"
)

// CompiledInBuild produces the two halves a compiled-in plugin registers:
// a native library and, when non-empty, the script source of the
// user-facing module. The native library registers under a twin name
// (scriptling._sqlite) and the script module under the public name
// (scriptling.sqlite), so scripts see exactly the namespace an external
// plugin of the same name would give them — and script-defined surfaces
// (like the ORM kit) execute host-side in both modes.
//
// policy is the host security context for the interpreter being configured;
// nil means no restrictions.
type CompiledInBuild func(policy *Policy) (*object.Library, string)

type compiledInEntry struct {
	name        string
	description string
	build       CompiledInBuild
}

var (
	compiledInMu    sync.RWMutex
	compiledInComps []compiledInEntry
)

// RegisterCompiledIn records a compiled-in plugin. It is called from init()
// in build-tag-guarded files (e.g. //go:build plugin_sqlite), so a binary
// only carries the plugins its build flags selected. Registering the same
// name twice panics: it means two tagged files disagree.
func RegisterCompiledIn(name, description string, build CompiledInBuild) {
	compiledInMu.Lock()
	defer compiledInMu.Unlock()
	for _, entry := range compiledInComps {
		if entry.name == name {
			panic("plugin: compiled-in plugin " + name + " registered twice")
		}
	}
	compiledInComps = append(compiledInComps, compiledInEntry{name: name, description: description, build: build})
}

// CompiledInNames returns the sorted names of all registered compiled-in
// plugins. Hosts use it to keep a discovered external plugin from shadowing
// (or being shadowed by) a compiled-in one.
func CompiledInNames() []string {
	compiledInMu.RLock()
	defer compiledInMu.RUnlock()
	names := make([]string, 0, len(compiledInComps))
	for _, entry := range compiledInComps {
		names = append(names, entry.name)
	}
	sort.Strings(names)
	return names
}

func compiledInGet(name string) (compiledInEntry, bool) {
	compiledInMu.RLock()
	defer compiledInMu.RUnlock()
	for _, entry := range compiledInComps {
		if entry.name == name {
			return entry, true
		}
	}
	return compiledInEntry{}, false
}

// registerCompiledIn registers every compiled-in plugin's library, built with
// the given policy. Compiled-ins register before discovered plugins so their
// names win; RegisterLibraries skips a discovered plugin whose declared name
// collides.
func registerCompiledIn(registrar Registrar, policy *Policy) {
	if registrar == nil {
		return
	}
	scriptRegistrar, canScript := registrar.(ScriptLibraryRegistrar)
	compiledInMu.RLock()
	entries := make([]compiledInEntry, len(compiledInComps))
	copy(entries, compiledInComps)
	compiledInMu.RUnlock()
	for _, entry := range entries {
		lib, script := entry.build(policy)
		if lib == nil {
			continue
		}
		registrar.RegisterLibrary(lib)
		if script == "" || !canScript {
			continue
		}
		if name := publicModuleName(lib.Name()); name != "" {
			_ = scriptRegistrar.RegisterScriptLibrary(name, script)
		}
	}
}

// publicModuleName turns a twin library name (scriptling._sqlite) into the
// user-facing module name (scriptling.sqlite); names without an
// underscored final segment return themselves unchanged.
func publicModuleName(twin string) string {
	dot := strings.LastIndex(twin, ".")
	if dot < 0 || dot+2 > len(twin) || twin[dot+1] != '_' {
		return twin
	}
	return twin[:dot+1] + twin[dot+2:]
}
