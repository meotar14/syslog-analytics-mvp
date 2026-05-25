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
	"syslog-analytics-mvp/internal/settings"
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

	var archiveStore *storage.PostgresArchiveStore
	if cfg.ArchiveHotPostgresDSN != "" {
		archiveStore, err = storage.NewPostgresArchiveStore(cfg.ArchiveHotPostgresDSN, cfg.ArchivePriorityPostgresDSN, cfg.ArchivePrioritySeverityMax)
		if err != nil {
			log.Fatalf("open archive postgres: %v", err)
		}
		defer archiveStore.Close()
	}

	collector := stats.NewCollector()
	if err := db.LoadSnapshot(collector); err != nil {
		log.Fatalf("load snapshot: %v", err)
	}
	storedSettings, err := db.LoadSettings(cfg.Retention)
	if err != nil {
		log.Fatalf("load settings: %v", err)
	}
	runtimeSettings := settings.New(storedSettings.Retention)

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
				snapshot := collector.Drain()
				if err := db.Flush(snapshot); err != nil {
					log.Printf("final flush failed: %v", err)
				}
				if archiveStore != nil {
					if err := archiveStore.Flush(snapshot); err != nil {
						log.Printf("final archive flush failed: %v", err)
					}
				}
				return
			case <-ticker.C:
				snapshot := collector.Drain()
				if err := db.Flush(snapshot); err != nil {
					log.Printf("flush failed: %v", err)
				}
				if err := db.ApplyRetention(runtimeSettings.Retention()); err != nil {
					log.Printf("retention failed: %v", err)
				}
				if archiveStore != nil {
					if err := archiveStore.Flush(snapshot); err != nil {
						log.Printf("archive flush failed: %v", err)
					}
					if err := archiveStore.ApplyRetention(cfg.ArchiveHotRetentionDays, cfg.ArchivePriorityRetentionDays); err != nil {
						log.Printf("archive retention failed: %v", err)
					}
				}
			}
		}
	}()

	if err := ingest.StartUDP(ctx, cfg.UDPListenAddr, collector); err != nil {
		log.Fatalf("udp listener: %v", err)
	}
	if err := ingest.StartTCP(ctx, cfg.TCPListenAddr, collector); err != nil {
		log.Fatalf("tcp listener: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           api.NewServer(cfg, db, archiveStore, collector, runtimeSettings),
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
