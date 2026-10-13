// Command scriptling built with the scriptling-onepassword extension, for the
// scriptling-onepassword and scriptling-slim-onepassword release variants.
//
// This is its own module so Scriptling's go.mod never depends on the
// 1Password SDK; it builds the CLI from the repository root through the
// replace in go.mod.
package main

import (
	_ "github.com/paularlott/scriptling-onepassword"
	"github.com/paularlott/scriptling/scriptling-cli/scriptlingcmd"
)

func main() {
	scriptlingcmd.Main()
}
