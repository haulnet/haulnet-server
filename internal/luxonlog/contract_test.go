package luxonlog

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Recorded output of the LuxonServer release pinned in luxon.lock (niansa v1.5). The port-busy run had UDP 27000
// held by another socket; the start run used the config RenderLuxonConfig writes. v1.5 keeps running after
// "Failed to bind", so this watcher is what stops haulnetd.

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
		"v1.5-linux-port-busy.stdout.log": "Failed to bind NameServer to port 27000!",
	} {
		got, ok := watch(t, name)
		if !ok || got != want {
			t.Errorf("%s: reported %q (%v), want %q", name, got, ok, want)
		}
	}
}

func TestRecordedStartOutputIsNotReported(t *testing.T) {
	for _, name := range []string{"v1.5-windows-start.stdout.log"} {
		if got, ok := watch(t, name); ok {
			t.Errorf("%s: unexpected failure %q", name, got)
		}
	}
}
