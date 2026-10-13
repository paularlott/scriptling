// Command scriptling is the Scriptling command line. Its logic lives in the
// scriptlingcmd package so custom builds can compile in extensions; see
// scriptlingcmd.Main.
package main

import "github.com/paularlott/scriptling/scriptling-cli/scriptlingcmd"

func main() {
	scriptlingcmd.Main()
}
