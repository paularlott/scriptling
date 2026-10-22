package container

import "os"

// appleCLI returns the Apple container CLI's fixed path: the operator's
// AppleCLI setting or DefaultAppleCLI. It is never looked up on PATH.
func appleCLI() string {
	if overrideAppleCLI != "" {
		return overrideAppleCLI
	}
	return DefaultAppleCLI
}

// appleCLIInstalled reports whether an executable is installed at the Apple
// CLI's fixed path. The path is the operator's, never a script's.
func appleCLIInstalled() bool {
	info, err := os.Stat(appleCLI())
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
