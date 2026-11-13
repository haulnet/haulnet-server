// Package status reads room and player counts from LuxonServer's local web API and publishes them, and
// nothing else, on a small read-only HTTP endpoint.
//
// LuxonServer's own web API must never be exposed: it has no authentication and returns player IP
// addresses, user IDs, auth tokens, room names and room properties. This package only forwards counts.
package status

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// voiceSuffix marks the Photon Voice room that R.E.P.O. opens next to every game room.
const voiceSuffix = "_voice_"

// cacheFor limits how often a status request reaches LuxonServer, so the public endpoint cannot be used to
// load it.
const cacheFor = 2 * time.Second

// Counts is what LuxonServer reports, summed over all apps.
type Counts struct {
	Rooms       int
	Players     int
	VoiceRooms  int
	Connections int
	BytesIn     uint64
	BytesOut    uint64
	CPUBusy     float64
}

// Report is the public status document.
type Report struct {
	HaulnetServer string  `json:"haulnet_server"`
	Luxon         string  `json:"luxon"`
	Name          string  `json:"name"`
	Rooms         int     `json:"rooms"`
	Players       int     `json:"players"`
	MaxRooms      int     `json:"max_rooms"`
	MaxPlayers    int     `json:"max_players"`
	CPUBusy       float64 `json:"cpu_busy"`
	Accepting     bool    `json:"accepting"`
	UptimeSeconds int64   `json:"uptime_s"`
}

// Luxon reads LuxonServer's web API on loopback.
type Luxon struct {
	BaseURL string
	Client  *http.Client
}

type appInfo struct {
	ID string `json:"id"`
}

type lobbyInfo struct {
	Name string `json:"name"`
}

type gameInfo struct {
	ID          string `json:"id"`
	PlayerCount int    `json:"player_count"`
}

type connectionInfo struct {
	Stats struct {
		BytesIn  uint64 `json:"bytes_in"`
		BytesOut uint64 `json:"bytes_out"`
	} `json:"stats"`
}

func (l *Luxon) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.BaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := l.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	return nil
}

// Ping reports whether LuxonServer's web API answers.
func (l *Luxon) Ping(ctx context.Context) error {
	var apps []appInfo
	return l.get(ctx, "/apps", &apps)
}

// Collect sums rooms and players over all apps. Rooms whose name ends with the voice suffix are counted
// separately: they belong to a game room and their players are already counted there.
func (l *Luxon) Collect(ctx context.Context) (Counts, error) {
	var c Counts

	var apps []appInfo
	if err := l.get(ctx, "/apps", &apps); err != nil {
		return c, err
	}
	for _, app := range apps {
		var lobbies []lobbyInfo
		if err := l.get(ctx, "/apps/"+url.PathEscape(app.ID)+"/lobbies", &lobbies); err != nil {
			return c, err
		}
		for _, lobby := range lobbies {
			var games []gameInfo
			path := "/apps/" + url.PathEscape(app.ID) + "/lobbies/" + url.PathEscape(lobby.Name) + "/games"
			if err := l.get(ctx, path, &games); err != nil {
				return c, err
			}
			for _, g := range games {
				if strings.HasSuffix(g.ID, voiceSuffix) {
					c.VoiceRooms++
					continue
				}
				c.Rooms++
				c.Players += g.PlayerCount
			}
		}
	}

	var conns []connectionInfo
	if err := l.get(ctx, "/connections", &conns); err != nil {
		return c, err
	}
	c.Connections = len(conns)
	for _, conn := range conns {
		c.BytesIn += conn.Stats.BytesIn
		c.BytesOut += conn.Stats.BytesOut
	}

	// Microseconds per server loop iteration spent working and waiting, averaged over one minute.
	var busy, idle float64
	if err := l.get(ctx, "/stats/busy_time/avg/60000", &busy); err != nil {
		return c, err
	}
	if err := l.get(ctx, "/stats/idle_time/avg/60000", &idle); err != nil {
		return c, err
	}
	if busy+idle > 0 {
		c.CPUBusy = busy / (busy + idle)
	}
	return c, nil
}

// Source provides counts; *Luxon in production.
type Source interface {
	Collect(context.Context) (Counts, error)
	Ping(context.Context) error
}

// Service caches counts and serves the public endpoints.
type Service struct {
	Source       Source
	Version      string
	LuxonVersion string
	Name         string
	MaxRooms     int
	MaxPlayers   int
	Started      time.Time
	Now          func() time.Time

	mu        sync.Mutex
	cached    Counts
	cachedAt  time.Time
	cachedErr error
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Counts returns cached counts, refreshing them at most every cacheFor.
func (s *Service) Counts(ctx context.Context) (Counts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cachedAt.IsZero() && s.now().Sub(s.cachedAt) < cacheFor {
		return s.cached, s.cachedErr
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	s.cached, s.cachedErr = s.Source.Collect(ctx)
	s.cachedAt = s.now()
	return s.cached, s.cachedErr
}

// Report builds the public status document.
func (s *Service) Report(ctx context.Context) (Report, error) {
	c, err := s.Counts(ctx)
	if err != nil {
		return Report{}, err
	}
	accepting := (s.MaxRooms == 0 || c.Rooms < s.MaxRooms) && (s.MaxPlayers == 0 || c.Players < s.MaxPlayers)
	return Report{
		HaulnetServer: s.Version,
		Luxon:         s.LuxonVersion,
		Name:          s.Name,
		Rooms:         c.Rooms,
		Players:       c.Players,
		MaxRooms:      s.MaxRooms,
		MaxPlayers:    s.MaxPlayers,
		CPUBusy:       float64(int(c.CPUBusy*1000)) / 1000,
		Accepting:     accepting,
		UptimeSeconds: int64(s.now().Sub(s.Started).Seconds()),
	}, nil
}

// Handler serves GET /status and GET /healthz. Everything else is 404.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		report, err := s.Report(r.Context())
		if err != nil {
			http.Error(w, "server is not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(report)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Source.Ping(ctx); err != nil {
			http.Error(w, "luxon server is not answering", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/", http.NotFound)
	return mux
}
