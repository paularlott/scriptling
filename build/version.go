// Package build contains build-time version information for Scriptling
package build

// Version is the current version of Scriptling
// This should be updated for each release
const Version = "1.0.0"

// BuildDate can be set at compile time using ldflags
// e.g., go build -ldflags "-X github.com/paularlott/scriptling/build.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var BuildDate = "unknown"

// GitCommit can be set at compile time using ldflags
// e.g., go build -ldflags "-X github.com/paularlott/scriptling/build.GitCommit=$(git rev-parse --short HEAD)"
var GitCommit = "unknown"
