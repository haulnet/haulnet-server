package status

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeLuxon answers like LuxonServer v1.5's web API (src/http_server.cpp) with one game room, its voice
// room and a hidden private room.
func fakeLuxon(t *testing.T) *httptest.Server {
	t.Helper()
	routes := map[string]string{
		"/apps":                           `[{"id":"game-app","version":"1.0","lobbies_count":1},{"id":"voice-app","version":"1.0","lobbies_count":1}]`,
		"/apps/game-app/lobbies":          `[{"name":"0","type":0,"games_count":2,"peer_count":5}]`,
		"/apps/game-app/lobbies/0/games":  `[{"id":"109775241234","player_count":3,"max_players":6,"is_open":true,"is_visible":false,"lobby_props":{}},{"id":"109775249999","player_count":2,"max_players":6,"is_open":true,"is_visible":true,"lobby_props":{}}]`,
		"/apps/voice-app/lobbies":         `[{"name":"0","type":0,"games_count":1,"peer_count":3}]`,
		"/apps/voice-app/lobbies/0/games": `[{"id":"109775241234_voice_","player_count":3,"max_players":6,"is_open":true,"is_visible":false,"lobby_props":{}}]`,
		"/connections":                    `[{"address":"198.51.100.1:5000","stats":{"bytes_in":100,"bytes_out":400}},{"address":"198.51.100.2:5000","stats":{"bytes_in":50,"bytes_out":200}}]`,
		"/stats/busy_time/avg/60000":      `250`,
		"/stats/idle_time/avg/60000":      `750`,
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.Path]
		if !ok {
			http.Error(w, `{"error":"Endpoint not found"}`, http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
}

func TestCollect(t *testing.T) {
	srv := fakeLuxon(t)
	defer srv.Close()

	c, err := (&Luxon{BaseURL: srv.URL, Client: srv.Client()}).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Counts{Rooms: 2, Players: 5, VoiceRooms: 1, Connections: 2, BytesIn: 150, BytesOut: 600, CPUBusy: 0.25}
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
}

func TestStatusPublishesOnlyCounts(t *testing.T) {
	srv := fakeLuxon(t)
	defer srv.Close()

	svc := &Service{
		Source:       &Luxon{BaseURL: srv.URL, Client: srv.Client()},
		Version:      "0.1.0",
		LuxonVersion: "v1.5",
		Name:         "test-1",
		MaxRooms:     2,
		Started:      time.Now().Add(-time.Minute),
	}
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	body := rec.Body.String()
	for _, secret := range []string{"109775241234", "198.51.100", "game-app", "voice-app", "bytes"} {
		if strings.Contains(body, secret) {
			t.Errorf("status leaks %q: %s", secret, body)
		}
	}

	var r Report
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Rooms != 2 || r.Players != 5 || r.Accepting || r.Name != "test-1" || r.Luxon != "v1.5" || r.UptimeSeconds < 59 {
		t.Fatalf("unexpected report %+v", r)
	}
}

type countingSource struct {
	calls int
	err   error
}

func (s *countingSource) Collect(context.Context) (Counts, error) {
	s.calls++
	return Counts{Rooms: s.calls}, s.err
}

func (s *countingSource) Ping(context.Context) error { return s.err }

func TestCountsAreCached(t *testing.T) {
	now := time.Unix(1000, 0)
	src := &countingSource{}
	svc := &Service{Source: src, Now: func() time.Time { return now }}

	for i := 0; i < 5; i++ {
		_, _ = svc.Counts(context.Background())
	}
	if src.calls != 1 {
		t.Fatalf("Collect called %d times within the cache window", src.calls)
	}
	now = now.Add(3 * time.Second)
	_, _ = svc.Counts(context.Background())
	if src.calls != 2 {
		t.Fatalf("Collect not called again after the cache window (%d calls)", src.calls)
	}
}

func TestEndpointsWhenLuxonIsDown(t *testing.T) {
	svc := &Service{Source: &countingSource{err: errors.New("connection refused")}}
	for _, path := range []string{"/status", "/healthz"} {
		rec := httptest.NewRecorder()
		svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status %d, want 503", path, rec.Code)
		}
	}
}

func TestOnlyStatusAndHealthAreServed(t *testing.T) {
	svc := &Service{Source: &countingSource{}}
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/apps"},
		{http.MethodGet, "/connections"},
		{http.MethodPost, "/batch"},
		{http.MethodPost, "/status"},
	} {
		rec := httptest.NewRecorder()
		svc.Handler().ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status %d, want 404 or 405", c.method, c.path, rec.Code)
		}
	}
}
