// Package config turns the operator's environment into a validated server configuration and renders the
// LuxonServer config.yml from it.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Ports R.E.P.O. uses (Photon "alternative UDP ports"). The game connects to the NameServer on 27000 and
// receives the Master and Game server addresses from it, so these are fixed.
const (
	NameServerPort   = 27000
	MasterServerPort = 27001
	GameServerPort   = 27002
)

// Region is the only region the server offers: LuxonServer name "europe", Photon code "eu", the same as the
// server bundled in the mod. The name means nothing for a Haulnet server; it only has to match.
const Region = "europe"

// Config is everything haulnetd needs to run one server.
type Config struct {
	ExternalAddress  string
	ServerName       string
	MaxConnections   int
	MaxRooms         int
	MaxPlayers       int
	StatusAddress    string
	LuxonHTTPPort    int
	LuxonBinary      string
	WorkDir          string
	StatsLogInterval time.Duration
}

var hostnamePattern = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// Load reads the configuration from a lookup function (os.LookupEnv in production). It returns warnings for
// settings that work but are probably not what a public server wants.
func Load(lookup func(string) (string, bool)) (Config, []string, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		return def
	}

	var errs []error
	var warnings []string

	intValue := func(key string, def, min, max int) int {
		raw := get(key, strconv.Itoa(def))
		v, err := strconv.Atoi(raw)
		if err != nil || v < min || v > max {
			errs = append(errs, fmt.Errorf("%s must be a whole number from %d to %d, got %q", key, min, max, raw))
			return def
		}
		return v
	}

	cfg := Config{
		ExternalAddress: get("EXTERNAL_ADDRESS", ""),
		ServerName:      get("SERVER_NAME", "haulnet-server"),
		MaxConnections:  intValue("MAX_CONNECTIONS", 600, 1, 100000),
		MaxRooms:        intValue("MAX_ROOMS", 0, 0, 100000),
		MaxPlayers:      intValue("MAX_PLAYERS", 0, 0, 100000),
		StatusAddress:   get("STATUS_ADDRESS", ":27080"),
		LuxonHTTPPort:   intValue("LUXON_HTTP_PORT", 5088, 1, 65535),
		LuxonBinary:     get("LUXON_BIN", ""),
		WorkDir:         get("WORK_DIR", get("STATE_DIRECTORY", ".")),
	}

	if raw := get("STATS_LOG_INTERVAL", "0"); raw != "0" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < 10*time.Second {
			errs = append(errs, fmt.Errorf("STATS_LOG_INTERVAL must be a duration of at least 10s (e.g. 60s) or 0, got %q", raw))
		} else {
			cfg.StatsLogInterval = d
		}
	}

	if len(cfg.ServerName) > 64 {
		errs = append(errs, errors.New("SERVER_NAME must be at most 64 characters"))
	}

	if _, _, err := net.SplitHostPort(cfg.StatusAddress); err != nil {
		errs = append(errs, fmt.Errorf("STATUS_ADDRESS must be host:port or :port, got %q", cfg.StatusAddress))
	}

	switch w, err := checkExternalAddress(cfg.ExternalAddress); {
	case err != nil:
		errs = append(errs, err)
	case w != "":
		warnings = append(warnings, w)
	}

	return cfg, warnings, errors.Join(errs...)
}

// checkExternalAddress validates the address players use to reach this server. LuxonServer hands it to
// every client as the Master and Game server address, so a wrong value is the most common misconfiguration.
func checkExternalAddress(addr string) (warning string, err error) {
	if addr == "" {
		return "", errors.New("EXTERNAL_ADDRESS is required: the public IP address or DNS name players use to reach this server")
	}

	if ip := net.ParseIP(strings.Trim(addr, "[]")); ip != nil {
		switch {
		case ip.IsUnspecified():
			return "", fmt.Errorf("EXTERNAL_ADDRESS %q is a wildcard address; set the address players connect to", addr)
		case ip.IsLoopback():
			return fmt.Sprintf("EXTERNAL_ADDRESS %s is a loopback address: only players on this machine can use it", addr), nil
		case ip.IsPrivate() || ip.IsLinkLocalUnicast() || isSharedAddressSpace(ip):
			return fmt.Sprintf("EXTERNAL_ADDRESS %s is a private address: fine for LAN or VPN, not for a public server", addr), nil
		}
		return "", nil
	}

	if strings.ContainsAny(addr, ":/ ") || len(addr) > 253 || !hostnamePattern.MatchString(addr) {
		return "", fmt.Errorf("EXTERNAL_ADDRESS %q is neither an IP address nor a DNS name (no scheme, no port)", addr)
	}
	return "", nil
}

// isSharedAddressSpace reports 100.64.0.0/10 (carrier-grade NAT, also used by Tailscale).
func isSharedAddressSpace(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1]&0xC0 == 64
}

// endpoint formats host:port, bracketing IPv6 literals.
func endpoint(host string, port int) string {
	return net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port))
}

// RenderLuxonConfig returns LuxonServer's config.yml. Only the game's UDP ports are opened, and the web API
// listens on loopback only: it has no authentication and exposes player addresses, user IDs and tokens.
func RenderLuxonConfig(cfg Config) string {
	var b strings.Builder
	b.WriteString("# Generated by haulnetd on every start. Change the environment, not this file.\n")
	b.WriteString("Servers:\n")
	for _, s := range []struct {
		name string
		port int
	}{{"NameServer", NameServerPort}, {"MasterServer", MasterServerPort}, {"GameServer", GameServerPort}} {
		// Validated addresses contain no quotes or escapes; the quotes keep "[v6]:port" a plain string.
		fmt.Fprintf(&b, "  %s:\n    - port: %d\n      external_address: \"%s\"\n", s.name, s.port, endpoint(cfg.ExternalAddress, s.port))
	}
	b.WriteString("EnableIPv6: true\n")
	fmt.Fprintf(&b, "MaxConnections: %d\n", cfg.MaxConnections)
	b.WriteString("MaxGamePeers: 0\n")
	b.WriteString("TickTimeBudget: 2000\n")
	b.WriteString("NoBanner: true\n")
	fmt.Fprintf(&b, "Regions:\n  - %s\n", Region)
	fmt.Fprintf(&b, "HTTP:\n  enabled: true\n  address: 127.0.0.1\n  port: %d\n", cfg.LuxonHTTPPort)
	return b.String()
}

// LoadEnvFile reads KEY=VALUE lines (as systemd's EnvironmentFile and Docker's env_file do) and returns a
// lookup that prefers real environment variables over the file.
func LoadEnvFile(path string) (func(string) (string, bool), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, line)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
		values[strings.TrimSpace(key)] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return func(key string) (string, bool) {
		if v, ok := os.LookupEnv(key); ok {
			return v, true
		}
		v, ok := values[key]
		return v, ok
	}, nil
}
