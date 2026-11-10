//go:build !windows

package become

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// signalContext ends when the helper is told to stop.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
}

// terminate asks a process to stop.
func terminate(p *os.Process) { _ = p.Signal(syscall.SIGTERM) }

// interrupt is how a command is stopped at its timeout or with its script.
func interrupt(p *os.Process) error { return p.Signal(syscall.SIGTERM) }

func fileOwner(info os.FileInfo) (uid, gid int, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int(st.Uid), int(st.Gid), true
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
