package luxonlog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Recorded output of the LuxonServer binaries haulnetd can run: "v1.5-*" is the niansa v1.5 release pinned in
// luxon.lock, "fork-a1a2-*" the haulnet/LuxonServer fork with "fatal bind" (A1) and "exclusive bind" (A2). The
// port-busy runs had UDP 27000 held by another socket; the start runs used the config RenderLuxonConfig writes.
// The fork keeps v1.5's "Failed to bind" line and adds a fatal line on stderr and exit code 1, so this watcher
// stays correct for both until every pin has the fork.

func watch(t *testing.T, name string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	w := &Watcher{Out: &out}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("%s: output not forwarded unchanged", name)
	}
	select {
	case f := <-w.Failures():
		return f, true
	default:
		return "", false
	}
}

func TestRecordedPortBusyOutputIsReported(t *testing.T) {
	for name, want := range map[string]string{
		"v1.5-linux-port-busy.stdout.log":      "Failed to bind NameServer to port 27000!",
		"fork-a1a2-linux-port-busy.stdout.log": "Failed to bind NameServer to port 27000!",
		"fork-a1a2-linux-port-busy.stderr.log": "Failed to bind NameServer to UDP port 27000: Address in use",
	} {
		got, ok := watch(t, name)
		if !ok || got != want {
			t.Errorf("%s: reported %q (%v), want %q", name, got, ok, want)
		}
	}
}

func TestRecordedStartOutputIsNotReported(t *testing.T) {
	for _, name := range []string{"v1.5-windows-start.stdout.log", "fork-a1a2-windows-start.stdout.log"} {
		if got, ok := watch(t, name); ok {
			t.Errorf("%s: unexpected failure %q", name, got)
		}
	}
}
