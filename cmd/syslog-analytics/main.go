package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"syslog-analytics-mvp/internal/api"
	"syslog-analytics-mvp/internal/buildinfo"
	"syslog-analytics-mvp/internal/config"
	"syslog-analytics-mvp/internal/ingest"
	"syslog-analytics-mvp/internal/stats"
	"syslog-analytics-mvp/internal/storage"
)

func main() {
	cfg := config.Load()
	log.Printf("starting syslog-analytics version=%s commit=%s build_date=%s", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate)

	db, err := storage.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	collector := stats.NewCollector()
	if err := db.LoadSnapshot(collector); err != nil {
		log.Fatalf("load snapshot: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		ticker := time.NewTicker(cfg.FlushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				if err := db.Flush(collector.Drain()); err != nil {
					log.Printf("final flush failed: %v", err)
				}
				return
			case <-ticker.C:
				if err := db.Flush(collector.Drain()); err != nil {
					log.Printf("flush failed: %v", err)
				}
				if err := db.ApplyRetention(cfg.Retention); err != nil {
					log.Printf("retention failed: %v", err)
				}
			}
		}
	}()

	if err := ingest.StartUDP(ctx, cfg.UDPListenAddr, collector); err != nil {
		log.Fatalf("udp listener: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           api.NewServer(cfg, db, collector),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("dashboard listening on %s", cfg.HTTPListenAddr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	<-flushDone
}
