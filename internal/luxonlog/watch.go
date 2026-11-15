// Package luxonlog watches LuxonServer's console output for failures it does not act on itself.
//
// LuxonServer v1.5 logs "Failed to bind <server> to port <port>!" when a port is taken and then keeps running
// without that server (src/server_manager.cpp, ServerManager::start_servers and the HTTP server setup). Players
// cannot connect, yet the process and its web API look healthy. haulnetd stops instead, so Docker, systemd and
// haulnet-update see a failed server.
package luxonlog

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

const bindFailure = "Failed to bind "

// Watcher forwards everything written to it to Out and reports lines that mean LuxonServer could not open
// a port. It is safe for concurrent use (stdout and stderr may share one Watcher).
type Watcher struct {
	Out io.Writer

	mu       sync.Mutex
	partial  []byte
	failures chan string
	once     sync.Once
}

// Failures delivers the first bind failure line (later ones are dropped: one is enough to stop).
func (w *Watcher) Failures() <-chan string {
	w.init()
	return w.failures
}

func (w *Watcher) init() {
	w.once.Do(func() { w.failures = make(chan string, 1) })
}

// Write forwards p to Out and scans complete lines.
func (w *Watcher) Write(p []byte) (int, error) {
	w.init()
	n, err := w.Out.Write(p)

	w.mu.Lock()
	defer w.mu.Unlock()
	w.partial = append(w.partial, p...)
	for {
		i := bytes.IndexByte(w.partial, '\n')
		if i < 0 {
			break
		}
		w.scan(string(w.partial[:i]))
		w.partial = w.partial[i+1:]
	}
	// A line without a newline that grows without end is not a log line we care about.
	if len(w.partial) > 64<<10 {
		w.partial = w.partial[:0]
	}
	return n, err
}

func (w *Watcher) scan(line string) {
	i := strings.Index(line, bindFailure)
	if i < 0 {
		return
	}
	select {
	case w.failures <- strings.TrimSpace(line[i:]):
	default:
	}
}
