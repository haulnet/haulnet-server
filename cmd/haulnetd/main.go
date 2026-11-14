// Command haulnetd runs a Haulnet community server: it renders LuxonServer's config from the environment,
// runs the unmodified LuxonServer binary as a child process and serves a read-only status endpoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/haulnet/haulnet-server/internal/config"
	"github.com/haulnet/haulnet-server/internal/status"
)

// Set at build time with -ldflags "-X main.version=... -X main.luxonVersion=...".
var (
	version      = "dev"
	luxonVersion = "v1.5"
)

func main() {
	envFile := flag.String("env", "", "read settings from this KEY=VALUE file (environment variables win)")
	printConfig := flag.Bool("print-config", false, "print the generated LuxonServer config and exit")
	showVersion := flag.Bool("version", false, "print versions and exit")
	healthcheck := flag.Bool("healthcheck", false, "ask the running server's /healthz and exit 0 if healthy (for Docker)")
	flag.Parse()

	log.SetFlags(log.LstdFlags)
	log.SetPrefix("haulnetd: ")

	if *showVersion {
		fmt.Printf("haulnet-server %s, LuxonServer %s\n", version, luxonVersion)
		return
	}

	lookup := os.LookupEnv
	if *envFile != "" {
		l, err := config.LoadEnvFile(*envFile)
		if err != nil {
			log.Fatalf("cannot read %s: %v", *envFile, err)
		}
		lookup = l
	}

	if *healthcheck {
		os.Exit(checkHealth(lookup))
	}

	cfg, warnings, err := config.Load(lookup)
	if err != nil {
		log.Fatalf("configuration error:\n%v", err)
	}
	for _, w := range warnings {
		log.Printf("warning: %s", w)
	}

	luxonConfig := config.RenderLuxonConfig(cfg)
	if *printConfig {
		fmt.Print(luxonConfig)
		return
	}

	os.Exit(run(cfg, luxonConfig))
}

func run(cfg config.Config, luxonConfig string) int {
	binary, err := findLuxon(cfg.LuxonBinary)
	if err != nil {
		log.Print(err)
		return 1
	}

	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		log.Printf("cannot create work directory %s: %v", cfg.WorkDir, err)
		return 1
	}
	// LuxonServer reads config.yml from its working directory.
	if err := os.WriteFile(filepath.Join(cfg.WorkDir, "config.yml"), []byte(luxonConfig), 0o644); err != nil {
		log.Printf("cannot write config.yml: %v", err)
		return 1
	}

	statusListener, err := net.Listen("tcp", cfg.StatusAddress)
	if err != nil {
		log.Printf("cannot open the status endpoint on %s: %v", cfg.StatusAddress, err)
		return 1
	}

	cmd := exec.Command(binary)
	cmd.Dir = cfg.WorkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("cannot start LuxonServer (%s): %v", binary, err)
		return 1
	}
	log.Printf("haulnet-server %s started LuxonServer %s (pid %d), external address %s, UDP %d-%d",
		version, luxonVersion, cmd.Process.Pid, cfg.ExternalAddress, config.NameServerPort, config.GameServerPort)

	luxon := &status.Luxon{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", cfg.LuxonHTTPPort),
		Client:  &http.Client{Timeout: 2 * time.Second},
	}
	svc := &status.Service{
		Source:       luxon,
		Version:      version,
		LuxonVersion: luxonVersion,
		Name:         cfg.ServerName,
		MaxRooms:     cfg.MaxRooms,
		MaxPlayers:   cfg.MaxPlayers,
		Started:      time.Now(),
		DrainFile:    filepath.Join(cfg.WorkDir, "drain"),
	}
	// A drain file left from an earlier run (systemd StateDirectory survives restarts) must not keep a fresh
	// server closed for new games.
	_ = os.Remove(svc.DrainFile)
	server := &http.Server{
		Handler:           svc.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}
	go func() {
		if err := server.Serve(statusListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("status endpoint stopped: %v", err)
		}
	}()
	log.Printf("status endpoint on http://%s/status", statusListener.Addr())

	ctx, stopStats := context.WithCancel(context.Background())
	defer stopStats()
	if cfg.StatsLogInterval > 0 {
		go logStats(ctx, luxon, cfg.StatsLogInterval)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	var waitErr error
	select {
	case waitErr = <-exited:
		log.Printf("LuxonServer exited: %v", describeExit(waitErr))
	case sig := <-signals:
		log.Printf("received %v, stopping LuxonServer", sig)
		stopChild(cmd)
		select {
		case waitErr = <-exited:
		case <-time.After(10 * time.Second):
			log.Print("LuxonServer did not stop in 10 s, killing it")
			_ = cmd.Process.Kill()
			waitErr = <-exited
		}
		waitErr = nil // A requested stop is a clean exit.
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)

	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
		return 0
	case errors.As(waitErr, &exitErr) && exitErr.ExitCode() > 0:
		return exitErr.ExitCode()
	default:
		return 1
	}
}

// checkHealth queries the local status endpoint. It needs only STATUS_ADDRESS, so it works without the
// rest of the configuration.
func checkHealth(lookup func(string) (string, bool)) int {
	addr, ok := lookup("STATUS_ADDRESS")
	if !ok || addr == "" {
		addr = ":27080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad STATUS_ADDRESS %q\n", addr)
		return 1
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthz: HTTP %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

// findLuxon returns the LuxonServer binary: LUXON_BIN, or luxon_server next to haulnetd.
func findLuxon(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err != nil {
			return "", fmt.Errorf("LUXON_BIN %s: %v", configured, err)
		}
		return configured, nil
	}
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate haulnetd: %v", err)
	}
	name := "luxon_server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(filepath.Dir(self), name)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("LuxonServer not found at %s; put it next to haulnetd or set LUXON_BIN", path)
	}
	return path, nil
}

func stopChild(cmd *exec.Cmd) {
	// Windows cannot deliver SIGTERM to a child; LuxonServer keeps no state worth a graceful stop there.
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
}

func describeExit(err error) string {
	if err == nil {
		return "exit code 0"
	}
	return err.Error()
}

// logStats logs aggregate load for capacity measurements. Byte counters belong to current connections, so a
// disconnect makes the sum drop; such intervals are reported as unknown instead of negative.
func logStats(ctx context.Context, luxon *status.Luxon, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	var prev status.Counts
	var prevAt time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		c, err := luxon.Collect(reqCtx)
		cancel()
		if err != nil {
			log.Printf("stats: %v", err)
			continue
		}

		now := time.Now()
		rates := "in/out n/a"
		if !prevAt.IsZero() && c.BytesIn >= prev.BytesIn && c.BytesOut >= prev.BytesOut {
			secs := now.Sub(prevAt).Seconds()
			rates = fmt.Sprintf("in %.1f kbit/s, out %.1f kbit/s",
				float64(c.BytesIn-prev.BytesIn)*8/1000/secs, float64(c.BytesOut-prev.BytesOut)*8/1000/secs)
		}
		log.Printf("stats: rooms %d, players %d, voice rooms %d, connections %d, %s, cpu busy %.1f%%",
			c.Rooms, c.Players, c.VoiceRooms, c.Connections, rates, c.CPUBusy*100)
		prev, prevAt = c, now
	}
}
