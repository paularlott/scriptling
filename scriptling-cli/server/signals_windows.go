//go:build windows

package server

import (
	"syscall"
)

var reloadSignals = []syscall.Signal{}

func isReloadSignal(sig syscall.Signal) bool {
	return false
}

func getReloadMessage() string {
	return "Press Ctrl+C to exit, tools auto-reload on file changes"
}
