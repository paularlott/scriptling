package main

import (
	"context"

	"github.com/paularlott/cli"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// checkScriptMetadata verifies the script's inline metadata block (PEP
// 723-style "# /// script") against the fully wired environment; the shared
// check lives in setup.CheckScriptMetadata, so one-shot runs, server setup
// scripts, and package main entries all verdict identically. It runs after
// plugins and packages are set up and before the script executes. Sources
// without a block are untouched.
func checkScriptMetadata(ctx context.Context, cmd *cli.Command, p *scriptling.Scriptling, packLoader *pack.Loader) error {
	var source []byte
	if code := cmd.GetString("code"); code != "" {
		source = []byte(code)
	} else if file := cmd.GetStringArg("file"); file != "" {
		var err error
		if isFetchedScript(file) {
			source, err = fetchScriptSource(ctx, file)
		} else {
			source, err = readFile(file)
		}
		if err != nil {
			return err
		}
	} else {
		// Stdin, interactive, and package main entries have no argument
		// script; package entries are checked where they resolve.
		return nil
	}
	return setup.CheckScriptMetadata(p, packLoader, pluginManager, source)
}
