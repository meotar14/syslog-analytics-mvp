package config

import (
	"os"
	"strconv"
	"time"
)

type Retention struct {
	SecondsDays int64
	MinutesDays int64
	HoursDays   int64
	DaysDays    int64
}

type Config struct {
	DBPath         string
	HTTPListenAddr string
	UDPListenAddr  string
	FlushInterval  time.Duration
	Retention      Retention
}

func Load() Config {
	return Config{
		DBPath:         getEnv("DB_PATH", "/data/syslog-analytics.db"),
		HTTPListenAddr: getEnv("HTTP_LISTEN_ADDR", ":8080"),
		UDPListenAddr:  getEnv("UDP_LISTEN_ADDR", ":5514"),
		FlushInterval:  time.Duration(getEnvInt("FLUSH_INTERVAL_SECONDS", 5)) * time.Second,
		Retention: Retention{
			SecondsDays: getEnvInt64("RETENTION_SECONDS_DAYS", 7),
			MinutesDays: getEnvInt64("RETENTION_MINUTES_DAYS", 30),
			HoursDays:   getEnvInt64("RETENTION_HOURS_DAYS", 365),
			DaysDays:    getEnvInt64("RETENTION_DAYS_DAYS", 3650),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}
