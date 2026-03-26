package api

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"syslog-analytics-mvp/internal/config"
	"syslog-analytics-mvp/internal/stats"
	"syslog-analytics-mvp/internal/storage"
)

//go:embed index.html
var assets embed.FS

type server struct {
	cfg       config.Config
	store     *storage.SQLiteStore
	collector *stats.Collector
	mux       *http.ServeMux
}

func NewServer(cfg config.Config, store *storage.SQLiteStore, collector *stats.Collector) http.Handler {
	s := &server{
		cfg:       cfg,
		store:     store,
		collector: collector,
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s.mux
}

func (s *server) routes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/overview", s.handleOverview)
	s.mux.HandleFunc("/api/timeseries", s.handleTimeSeries)
	s.mux.HandleFunc("/api/sources", s.handleSources)
	s.mux.HandleFunc("/api/severity", s.handleSeverity)
	s.mux.HandleFunc("/api/facility", s.handleFacility)
	s.mux.HandleFunc("/api/health", s.handleHealth)
}

func (s *server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	body, err := assets.ReadFile("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
}

func (s *server) handleOverview(w http.ResponseWriter, r *http.Request) {
	rangeMinutes := readRangeMinutes(r, 24*60)
	data, err := s.store.Overview(rangeMinutes)
	writeJSON(w, data, err)
}

func (s *server) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	rangeMinutes := readRangeMinutes(r, 24*60)
	data, err := s.store.TimeSeries(rangeMinutes)
	writeJSON(w, data, err)
}

func (s *server) handleSources(w http.ResponseWriter, r *http.Request) {
	rangeMinutes := readRangeMinutes(r, 24*60)
	limit := 10
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	data, err := s.store.TopSources(rangeMinutes, limit)
	writeJSON(w, data, err)
}

func (s *server) handleSeverity(w http.ResponseWriter, r *http.Request) {
	rangeMinutes := readRangeMinutes(r, 24*60)
	data, err := s.store.SeverityBreakdown(rangeMinutes)
	writeJSON(w, data, err)
}

func (s *server) handleFacility(w http.ResponseWriter, r *http.Request) {
	rangeMinutes := readRangeMinutes(r, 24*60)
	data, err := s.store.FacilityBreakdown(rangeMinutes)
	writeJSON(w, data, err)
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	payload := map[string]any{
		"status":          "ok",
		"started_at":      s.collector.StartedAt().Unix(),
		"http_listen":     s.cfg.HTTPListenAddr,
		"udp_listen":      s.cfg.UDPListenAddr,
		"flush_interval":  s.cfg.FlushInterval.String(),
		"storage_backend": "sqlite",
	}
	writeJSON(w, payload, nil)
}

func readRangeMinutes(r *http.Request, fallback int64) int64 {
	raw := r.URL.Query().Get("range_minutes")
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, payload any, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		log.Printf("api error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}
