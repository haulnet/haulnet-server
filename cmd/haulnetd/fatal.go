package main

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

// fatalPrefix starts the line LuxonServer (Haulnet's fork) writes to stderr before it exits with code 1 because it
// could not start: a port that cannot be bound, a config error. Only the prefix is matched; the reason text comes
// from the C library and differs between builds ("address in use" on Windows, "Address in use" with musl).
const fatalPrefix = "Fatal error: "

// fatalLine forwards LuxonServer's stderr to Out and keeps its fatal message, so haulnetd can say why the server
// stopped instead of only "exit status 1".
type fatalLine struct {
	Out io.Writer

	mu      sync.Mutex
	partial []byte
	message string
}

func (f *fatalLine) Write(p []byte) (int, error) {
	n, err := f.Out.Write(p)

	f.mu.Lock()
	defer f.mu.Unlock()
	f.partial = append(f.partial, p...)
	for {
		i := bytes.IndexByte(f.partial, '\n')
		if i < 0 {
			break
		}
		f.scan(string(f.partial[:i]))
		f.partial = f.partial[i+1:]
	}
	if len(f.partial) > 64<<10 {
		f.partial = f.partial[:0]
	}
	return n, err
}

func (f *fatalLine) scan(line string) {
	line = strings.TrimSpace(line)
	if f.message == "" && strings.HasPrefix(line, fatalPrefix) {
		f.message = strings.TrimPrefix(line, fatalPrefix)
	}
}

// Message returns the first fatal message, or "" when LuxonServer wrote none.
func (f *fatalLine) Message() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.message
}

// explainFailure turns LuxonServer's fatal message into haulnetd's log line.
func explainFailure(message string) string {
	text := "LuxonServer could not start: " + message
	if strings.Contains(message, "Failed to bind ") {
		text += ". Another program is probably using the port; players cannot connect, so haulnet-server stops. " +
			"Free the port and start it again."
	}
	return text
}
