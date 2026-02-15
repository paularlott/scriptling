//go:build !windows

package server

import (
	"syscall"
)

var reloadSignals = []syscall.Signal{syscall.SIGHUP, syscall.SIGUSR1}

func isReloadSignal(sig syscall.Signal) bool {
	return sig == syscall.SIGHUP || sig == syscall.SIGUSR1
}

func getReloadMessage() string {
	return "Press Ctrl+C to exit, tools auto-reload on file changes, SIGHUP/SIGUSR1 to force reload"
}
