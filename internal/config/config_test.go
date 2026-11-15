package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lookupFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, warnings, err := Load(lookupFrom(map[string]string{"EXTERNAL_ADDRESS": "203.0.113.10"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if cfg.MaxConnections != 600 || cfg.StatusAddress != ":27080" || cfg.LuxonHTTPPort != 5088 || cfg.WorkDir != "." {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestExternalAddress(t *testing.T) {
	cases := []struct {
		addr    string
		wantErr bool
		warn    bool
	}{
		{"", true, false},
		{"203.0.113.10", false, false},
		{"msk1.example.net", false, false},
		{"2001:db8::1", false, false},
		{"[2001:db8::1]", false, false},
		{"127.0.0.1", false, true},
		{"192.168.20.50", false, true},
		{"100.101.102.103", false, true},
		{"0.0.0.0", true, false},
		{"::", true, false},
		{"udp://203.0.113.10", true, false},
		{"203.0.113.10:27000", true, false},
		{"bad host", true, false},
		{"-bad.example", true, false},
	}
	for _, c := range cases {
		_, warnings, err := Load(lookupFrom(map[string]string{"EXTERNAL_ADDRESS": c.addr}))
		if (err != nil) != c.wantErr {
			t.Errorf("%q: err = %v, want error %v", c.addr, err, c.wantErr)
		}
		if (len(warnings) > 0) != c.warn {
			t.Errorf("%q: warnings = %v, want warning %v", c.addr, warnings, c.warn)
		}
	}
}

func TestInvalidNumbersAreReportedTogether(t *testing.T) {
	_, _, err := Load(lookupFrom(map[string]string{
		"EXTERNAL_ADDRESS": "203.0.113.10",
		"MAX_CONNECTIONS":  "lots",
		"MAX_ROOMS":        "-1",
		"STATUS_ADDRESS":   "27080",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"MAX_CONNECTIONS", "MAX_ROOMS", "STATUS_ADDRESS"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

func TestRenderLuxonConfig(t *testing.T) {
	out := RenderLuxonConfig(Config{ExternalAddress: "2001:db8::1", MaxConnections: 300, LuxonHTTPPort: 5088})
	for _, want := range []string{
		"  NameServer:\n    - port: 27000\n      external_address: \"[2001:db8::1]:27000\"\n",
		"  MasterServer:\n    - port: 27001\n",
		"  GameServer:\n    - port: 27002\n",
		"MaxConnections: 300\n",
		"Regions:\n  - europe\n",
		"HTTP:\n  enabled: true\n  address: 127.0.0.1\n  port: 5088\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("config does not contain %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"5055", "5056", "5058", "proxies", "0.0.0.0"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("config must not contain %q:\n%s", unwanted, out)
		}
	}
}

func TestLoadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.env")
	content := "# comment\nEXTERNAL_ADDRESS=\"198.51.100.7\"\n\nSERVER_NAME='Test 1'\nHAULNET_TEST_ENV_OVERRIDE=file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HAULNET_TEST_ENV_OVERRIDE", "env")

	lookup, err := LoadEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := lookup("EXTERNAL_ADDRESS"); v != "198.51.100.7" {
		t.Errorf("EXTERNAL_ADDRESS = %q", v)
	}
	if v, _ := lookup("SERVER_NAME"); v != "Test 1" {
		t.Errorf("SERVER_NAME = %q", v)
	}
	if v, _ := lookup("HAULNET_TEST_ENV_OVERRIDE"); v != "env" {
		t.Errorf("environment must win over the file, got %q", v)
	}

	bad := filepath.Join(t.TempDir(), "bad.env")
	_ = os.WriteFile(bad, []byte("NOT A PAIR\n"), 0o600)
	if _, err := LoadEnvFile(bad); err == nil {
		t.Error("expected an error for a line without =")
	}
}

// testdata/luxon-config.yml is the exact file LuxonServer v1.5 was started with in the contract checks (accepted
// without errors or warnings). A change here needs that check again.
func TestRenderLuxonConfigMatchesCheckedFile(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "luxon-config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(lookupFrom(map[string]string{"EXTERNAL_ADDRESS": "127.0.0.1"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := RenderLuxonConfig(cfg); got != string(want) {
		t.Errorf("config differs from testdata/luxon-config.yml:\n%s", got)
	}
}
