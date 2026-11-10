//go:build windows

package become

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/signal"
)

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func terminate(p *os.Process) { _ = p.Kill() }

func interrupt(p *os.Process) error { return p.Kill() }

func fileOwner(os.FileInfo) (int, int, bool) { return 0, 0, false }

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
