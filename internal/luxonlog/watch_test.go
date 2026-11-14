package luxonlog

import (
	"bytes"
	"testing"
)

func TestForwardsOutputUnchanged(t *testing.T) {
	var out bytes.Buffer
	w := &Watcher{Out: &out}
	input := "[2026-11-14 09:48:34.532] [ServerManager] [info] Starting NameServer on port 27000\npartial"
	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if out.String() != input {
		t.Fatalf("forwarded %q", out.String())
	}
	select {
	case f := <-w.Failures():
		t.Fatalf("unexpected failure %q", f)
	default:
	}
}

// The line LuxonServer v1.5 printed on the official server when another process held UDP 27000.
func TestReportsBindFailureSplitAcrossWrites(t *testing.T) {
	w := &Watcher{Out: &bytes.Buffer{}}
	_, _ = w.Write([]byte("[2026-11-14 09:48:24.648] [ServerManager] [error] Failed to bind Name"))
	select {
	case f := <-w.Failures():
		t.Fatalf("reported before the line was complete: %q", f)
	default:
	}
	_, _ = w.Write([]byte("Server to port 27000!\n[2026-11-14 09:48:24.649] [ServerManager] [error] Failed to bind HTTP server to port 5088! Is the port already in use?\n"))

	select {
	case f := <-w.Failures():
		if f != "Failed to bind NameServer to port 27000!" {
			t.Fatalf("failure %q", f)
		}
	default:
		t.Fatal("bind failure not reported")
	}
	select {
	case f := <-w.Failures():
		t.Fatalf("second failure should be dropped, got %q", f)
	default:
	}
}
