package storage

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"

	"syslog-analytics-mvp/internal/config"
	"syslog-analytics-mvp/internal/stats"
)

type SQLiteStore struct {
	db *sql.DB
}

type Overview struct {
	TotalMessages    int64   `json:"total_messages"`
	TotalBytes       int64   `json:"total_bytes"`
	AvgMessagesSec   float64 `json:"avg_messages_sec"`
	PeakMessagesSec  int64   `json:"peak_messages_sec"`
	UniqueSources    int64   `json:"unique_sources"`
	ParseFailureRate float64 `json:"parse_failure_rate"`
}

type TimePoint struct {
	TS       int64 `json:"ts"`
	MsgCount int64 `json:"msg_count"`
	ByteCount int64 `json:"byte_count"`
}

type SourceRow struct {
	SourceIP  string `json:"source_ip"`
	Hostname  string `json:"hostname"`
	MsgCount  int64  `json:"msg_count"`
	ByteCount int64  `json:"byte_count"`
}

type DimRow struct {
	Value     int   `json:"value"`
	MsgCount  int64 `json:"msg_count"`
	ByteCount int64 `json:"byte_count"`
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) init() error {
	stmts := []string{
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS stats_second (
			ts INTEGER PRIMARY KEY,
			msg_count INTEGER NOT NULL,
			byte_count INTEGER NOT NULL,
			parsed_ok_count INTEGER NOT NULL,
			parsed_fail_count INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS stats_minute (
			ts INTEGER PRIMARY KEY,
			msg_count INTEGER NOT NULL,
			byte_count INTEGER NOT NULL,
			parsed_ok_count INTEGER NOT NULL,
			parsed_fail_count INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS stats_hour (
			ts INTEGER PRIMARY KEY,
			msg_count INTEGER NOT NULL,
			byte_count INTEGER NOT NULL,
			parsed_ok_count INTEGER NOT NULL,
			parsed_fail_count INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS stats_day (
			ts INTEGER PRIMARY KEY,
			msg_count INTEGER NOT NULL,
			byte_count INTEGER NOT NULL,
			parsed_ok_count INTEGER NOT NULL,
			parsed_fail_count INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS stats_by_source_minute (
			ts INTEGER NOT NULL,
			source_ip TEXT NOT NULL,
			hostname TEXT NOT NULL,
			msg_count INTEGER NOT NULL,
			byte_count INTEGER NOT NULL,
			parsed_ok_count INTEGER NOT NULL,
			parsed_fail_count INTEGER NOT NULL,
			PRIMARY KEY (ts, source_ip, hostname)
		);`,
		`CREATE TABLE IF NOT EXISTS stats_by_severity_minute (
			ts INTEGER NOT NULL,
			severity INTEGER NOT NULL,
			msg_count INTEGER NOT NULL,
			byte_count INTEGER NOT NULL,
			PRIMARY KEY (ts, severity)
		);`,
		`CREATE TABLE IF NOT EXISTS stats_by_facility_minute (
			ts INTEGER NOT NULL,
			facility INTEGER NOT NULL,
			msg_count INTEGER NOT NULL,
			byte_count INTEGER NOT NULL,
			PRIMARY KEY (ts, facility)
		);`,
		`CREATE TABLE IF NOT EXISTS source_registry (
			source_ip TEXT PRIMARY KEY,
			last_hostname TEXT NOT NULL,
			first_seen_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL,
			total_msgs INTEGER NOT NULL,
			total_bytes INTEGER NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) LoadSnapshot(collector *stats.Collector) error {
	rows, err := s.db.Query(`SELECT source_ip, last_hostname, first_seen_at, last_seen_at, total_msgs, total_bytes FROM source_registry`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var reg stats.SourceSummary
		if err := rows.Scan(&reg.SourceIP, &reg.Hostname, &reg.FirstSeen, &reg.LastSeen, &reg.TotalMsgs, &reg.TotalByte); err != nil {
			return err
		}
		collector.RestoreSource(reg)
	}
	return rows.Err()
}

func (s *SQLiteStore) Flush(snapshot stats.Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := upsertCounterMap(tx, `INSERT INTO stats_second (ts, msg_count, byte_count, parsed_ok_count, parsed_fail_count)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(ts) DO UPDATE SET
		msg_count = msg_count + excluded.msg_count,
		byte_count = byte_count + excluded.byte_count,
		parsed_ok_count = parsed_ok_count + excluded.parsed_ok_count,
		parsed_fail_count = parsed_fail_count + excluded.parsed_fail_count`, snapshot.PerSecond); err != nil {
		return err
	}
	if err := upsertCounterMap(tx, `INSERT INTO stats_minute (ts, msg_count, byte_count, parsed_ok_count, parsed_fail_count)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(ts) DO UPDATE SET
		msg_count = msg_count + excluded.msg_count,
		byte_count = byte_count + excluded.byte_count,
		parsed_ok_count = parsed_ok_count + excluded.parsed_ok_count,
		parsed_fail_count = parsed_fail_count + excluded.parsed_fail_count`, snapshot.PerMinute); err != nil {
		return err
	}
	if err := upsertCounterMap(tx, `INSERT INTO stats_hour (ts, msg_count, byte_count, parsed_ok_count, parsed_fail_count)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(ts) DO UPDATE SET
		msg_count = msg_count + excluded.msg_count,
		byte_count = byte_count + excluded.byte_count,
		parsed_ok_count = parsed_ok_count + excluded.parsed_ok_count,
		parsed_fail_count = parsed_fail_count + excluded.parsed_fail_count`, snapshot.PerHour); err != nil {
		return err
	}
	if err := upsertCounterMap(tx, `INSERT INTO stats_day (ts, msg_count, byte_count, parsed_ok_count, parsed_fail_count)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(ts) DO UPDATE SET
		msg_count = msg_count + excluded.msg_count,
		byte_count = byte_count + excluded.byte_count,
		parsed_ok_count = parsed_ok_count + excluded.parsed_ok_count,
		parsed_fail_count = parsed_fail_count + excluded.parsed_fail_count`, snapshot.PerDay); err != nil {
		return err
	}

	for key, counter := range snapshot.PerSourceMinute {
		if _, err := tx.Exec(`INSERT INTO stats_by_source_minute (ts, source_ip, hostname, msg_count, byte_count, parsed_ok_count, parsed_fail_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(ts, source_ip, hostname) DO UPDATE SET
			msg_count = msg_count + excluded.msg_count,
			byte_count = byte_count + excluded.byte_count,
			parsed_ok_count = parsed_ok_count + excluded.parsed_ok_count,
			parsed_fail_count = parsed_fail_count + excluded.parsed_fail_count`,
			key.Minute, key.SourceIP, key.Hostname, counter.MsgCount, counter.ByteCount, counter.ParsedOKCount, counter.ParsedFailCount); err != nil {
			return err
		}
	}

	for key, counter := range snapshot.PerSeverityMinute {
		if _, err := tx.Exec(`INSERT INTO stats_by_severity_minute (ts, severity, msg_count, byte_count)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(ts, severity) DO UPDATE SET
			msg_count = msg_count + excluded.msg_count,
			byte_count = byte_count + excluded.byte_count`,
			key.Minute, key.Value, counter.MsgCount, counter.ByteCount); err != nil {
			return err
		}
	}

	for key, counter := range snapshot.PerFacilityMinute {
		if _, err := tx.Exec(`INSERT INTO stats_by_facility_minute (ts, facility, msg_count, byte_count)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(ts, facility) DO UPDATE SET
			msg_count = msg_count + excluded.msg_count,
			byte_count = byte_count + excluded.byte_count`,
			key.Minute, key.Value, counter.MsgCount, counter.ByteCount); err != nil {
			return err
		}
	}

	for _, reg := range snapshot.SourceRegistry {
		if _, err := tx.Exec(`INSERT INTO source_registry (source_ip, last_hostname, first_seen_at, last_seen_at, total_msgs, total_bytes)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(source_ip) DO UPDATE SET
			last_hostname = excluded.last_hostname,
			first_seen_at = MIN(first_seen_at, excluded.first_seen_at),
			last_seen_at = MAX(last_seen_at, excluded.last_seen_at),
			total_msgs = total_msgs + excluded.total_msgs,
			total_bytes = total_bytes + excluded.total_bytes`,
			reg.SourceIP, reg.Hostname, reg.FirstSeen, reg.LastSeen, reg.TotalMsgs, reg.TotalByte); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func upsertCounterMap(tx *sql.Tx, query string, values map[int64]stats.Counter) error {
	for ts, counter := range values {
		if _, err := tx.Exec(query, ts, counter.MsgCount, counter.ByteCount, counter.ParsedOKCount, counter.ParsedFailCount); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) ApplyRetention(r config.Retention) error {
	now := time.Now().UTC()
	type target struct {
		table string
		cut   int64
	}
	targets := []target{
		{"stats_second", now.Add(-time.Duration(r.SecondsDays) * 24 * time.Hour).Unix()},
		{"stats_minute", now.Add(-time.Duration(r.MinutesDays) * 24 * time.Hour).Unix()},
		{"stats_by_source_minute", now.Add(-time.Duration(r.MinutesDays) * 24 * time.Hour).Unix()},
		{"stats_by_severity_minute", now.Add(-time.Duration(r.MinutesDays) * 24 * time.Hour).Unix()},
		{"stats_by_facility_minute", now.Add(-time.Duration(r.MinutesDays) * 24 * time.Hour).Unix()},
		{"stats_hour", now.Add(-time.Duration(r.HoursDays) * 24 * time.Hour).Unix()},
		{"stats_day", now.Add(-time.Duration(r.DaysDays) * 24 * time.Hour).Unix()},
	}

	for _, item := range targets {
		if _, err := s.db.Exec(`DELETE FROM `+item.table+` WHERE ts < ?`, item.cut); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) Overview(rangeMinutes int64) (Overview, error) {
	start := time.Now().UTC().Add(-time.Duration(rangeMinutes) * time.Minute).Unix()
	row := s.db.QueryRow(`
		SELECT
			COALESCE(SUM(msg_count), 0),
			COALESCE(SUM(byte_count), 0),
			COALESCE(SUM(parsed_fail_count), 0),
			COALESCE(MAX(msg_count), 0)
		FROM stats_minute
		WHERE ts >= ?`, start)

	var out Overview
	var failed int64
	if err := row.Scan(&out.TotalMessages, &out.TotalBytes, &failed, &out.PeakMessagesSec); err != nil {
		return out, err
	}
	if rangeMinutes > 0 {
		out.AvgMessagesSec = float64(out.TotalMessages) / float64(rangeMinutes*60)
	}
	if out.TotalMessages > 0 {
		out.ParseFailureRate = float64(failed) / float64(out.TotalMessages)
	}

	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT source_ip) FROM stats_by_source_minute WHERE ts >= ?`, start).Scan(&out.UniqueSources); err != nil {
		return out, err
	}
	return out, nil
}

func (s *SQLiteStore) TimeSeries(rangeMinutes int64) ([]TimePoint, error) {
	start := time.Now().UTC().Add(-time.Duration(rangeMinutes) * time.Minute).Unix()
	rows, err := s.db.Query(`SELECT ts, msg_count, byte_count FROM stats_minute WHERE ts >= ? ORDER BY ts ASC`, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimePoint
	for rows.Next() {
		var point TimePoint
		if err := rows.Scan(&point.TS, &point.MsgCount, &point.ByteCount); err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) TopSources(rangeMinutes int64, limit int) ([]SourceRow, error) {
	start := time.Now().UTC().Add(-time.Duration(rangeMinutes) * time.Minute).Unix()
	rows, err := s.db.Query(`
		SELECT source_ip, hostname, SUM(msg_count), SUM(byte_count)
		FROM stats_by_source_minute
		WHERE ts >= ?
		GROUP BY source_ip, hostname
		ORDER BY SUM(msg_count) DESC
		LIMIT ?`, start, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SourceRow
	for rows.Next() {
		var row SourceRow
		if err := rows.Scan(&row.SourceIP, &row.Hostname, &row.MsgCount, &row.ByteCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SeverityBreakdown(rangeMinutes int64) ([]DimRow, error) {
	return s.dimensionBreakdown(`SELECT severity, SUM(msg_count), SUM(byte_count) FROM stats_by_severity_minute WHERE ts >= ? GROUP BY severity ORDER BY severity ASC`, rangeMinutes)
}

func (s *SQLiteStore) FacilityBreakdown(rangeMinutes int64) ([]DimRow, error) {
	return s.dimensionBreakdown(`SELECT facility, SUM(msg_count), SUM(byte_count) FROM stats_by_facility_minute WHERE ts >= ? GROUP BY facility ORDER BY facility ASC`, rangeMinutes)
}

func (s *SQLiteStore) dimensionBreakdown(query string, rangeMinutes int64) ([]DimRow, error) {
	start := time.Now().UTC().Add(-time.Duration(rangeMinutes) * time.Minute).Unix()
	rows, err := s.db.Query(query, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DimRow
	for rows.Next() {
		var row DimRow
		if err := rows.Scan(&row.Value, &row.MsgCount, &row.ByteCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
