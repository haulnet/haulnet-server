package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Recorded stderr of the pinned release v1.5-haulnet.1 (musl on Linux, MSVC on Windows) when it could not start.
func TestFatalMessageFromRecordedOutput(t *testing.T) {
	for name, want := range map[string]string{
		"release-linux-port-busy.stderr.log":    "Failed to bind NameServer to UDP port 27000: Address in use",
		"release-windows-port-busy.stderr.log":  "Failed to bind NameServer to UDP port 28000: address in use",
		"release-linux-config-error.stderr.log": "Config error at Foo: unknown top-level key: Foo",
	} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		f := &fatalLine{Out: &out}
		// Split the write to check that a line spread over two writes is still found.
		half := len(data) / 3
		_, _ = f.Write(data[:half])
		_, _ = f.Write(data[half:])
		if out.String() != string(data) {
			t.Errorf("%s: stderr not forwarded unchanged", name)
		}
		if got := f.Message(); got != want {
			t.Errorf("%s: message %q, want %q", name, got, want)
		}
	}
}

func TestNoFatalMessageWithoutTheLine(t *testing.T) {
	f := &fatalLine{Out: &bytes.Buffer{}}
	_, _ = f.Write([]byte("[2026-11-15 17:14:23.757] [ServerManager] [error] Failed to bind NameServer to port 27000!\nRuntime destroyed.\n"))
	if got := f.Message(); got != "" {
		t.Fatalf("unexpected message %q", got)
	}
}

func TestExplainFailure(t *testing.T) {
	bind := explainFailure("Failed to bind NameServer to UDP port 27000: Address in use")
	if !strings.HasPrefix(bind, "LuxonServer could not start: Failed to bind NameServer to UDP port 27000") || !strings.Contains(bind, "Free the port") {
		t.Errorf("bind failure: %q", bind)
	}
	if config := explainFailure("Config error at Foo: unknown top-level key: Foo"); strings.Contains(config, "port") {
		t.Errorf("config error mentions a port: %q", config)
	}
}
