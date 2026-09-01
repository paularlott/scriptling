// script-metadata shows an embedding host checking a script's inline
// metadata block before running it: parse the block, verify it against the
// host's real environment — the host's own version, the libraries and
// loaders it wired up, the plugins it loaded — and refuse to run anything
// with unmet requirements.
//
// Run it with:
//
//	go run ./examples/script-metadata
package main

import (
	"fmt"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/metadata"
)

// hostVersion is what requires-scriptling is checked against. It is the
// host's version, not scriptling's: from a script's perspective, the host is
// the interpreter it depends on.
const hostVersion = "1.4.0"

// hostutils is the library both scripts want; the host registers it
// natively, so it resolves without any plugin.
const utilsLibrary = `
def greeting(name):
    return "hello " + name
`

const goodScript = `# /// script
# requires-scriptling = ">=1.2"
# dependencies = ["hostutils"]
# ///
import hostutils
print(hostutils.greeting("embedded host"))
`

// failingScript wants knot, which this host never loads: the dependency does
// not resolve, its declared plugin is promoted, and the run is refused
// before a single statement executes.
const failingScript = `# /// script
# requires-scriptling = ">=1.2"
# dependencies = ["hostutils", "knot.space via knot >= 1.2"]
# ///
import hostutils
import knot.space
`

func main() {
	for _, source := range []string{goodScript, failingScript} {
		if err := run(source); err != nil {
			fmt.Printf("refused: %v\n", err)
		}
	}
}

func run(source string) error {
	p := scriptling.New()
	if err := p.RegisterScriptLibrary("hostutils", utilsLibrary); err != nil {
		return err
	}

	// The check runs against the fully wired interpreter, before the script
	// executes — the same position the CLI checks from.
	m, ok, err := metadata.Parse([]byte(source))
	if err != nil {
		return fmt.Errorf("malformed metadata block: %w", err)
	}
	if ok {
		err = m.Verify(metadata.Env{
			HostVersion: hostVersion,
			Resolves:    p.HasLibrary,
			PluginVersion: func(name string) (string, bool) {
				// This host embeds no plugins, so every plugins entry and
				// every unresolved via clause reports "not loaded".
				return "", false
			},
		})
		if err != nil {
			return err
		}
	}

	_, err = p.Eval(source)
	return err
}
