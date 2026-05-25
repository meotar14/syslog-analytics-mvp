package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"syslog-analytics-mvp/internal/stats"
)

type PostgresArchiveStore struct {
	hotDB               *sql.DB
	priorityDB          *sql.DB
	prioritySeverityMax int
	mu                  sync.Mutex
	knownPartitions     map[string]struct{}
}

type ArchiveQuery struct {
	Start    time.Time
	End      time.Time
	SourceIP string
	Hostname string
	Severity *int
	Vendor   string
	Limit    int
}

type ArchivedLogEvent struct {
	ReceivedAt        time.Time `json:"received_at"`
	SourceIP          string    `json:"source_ip"`
	Hostname          string    `json:"hostname"`
	Severity          *int      `json:"severity,omitempty"`
	Facility          *int      `json:"facility,omitempty"`
	Program           string    `json:"program"`
	Vendor            string    `json:"vendor"`
	ParsedOK          bool      `json:"parsed_ok"`
	Message           string    `json:"message"`
	RawMessage        string    `json:"raw_message"`
	FortiGateType     string    `json:"fortigate_type,omitempty"`
	FortiGateSubtype  string    `json:"fortigate_subtype,omitempty"`
	FortiGateLevel    string    `json:"fortigate_level,omitempty"`
	FortiGateAction   string    `json:"fortigate_action,omitempty"`
	FortiGateSrcIP    string    `json:"fortigate_srcip,omitempty"`
	FortiGateDstIP    string    `json:"fortigate_dstip,omitempty"`
	FortiGateUserName string    `json:"fortigate_user_name,omitempty"`
}

func NewPostgresArchiveStore(hotDSN, priorityDSN string, prioritySeverityMax int) (*PostgresArchiveStore, error) {
	hotDB, err := sql.Open("pgx", hotDSN)
	if err != nil {
		return nil, err
	}

	priorityDB := hotDB
	if priorityDSN != "" && priorityDSN != hotDSN {
		priorityDB, err = sql.Open("pgx", priorityDSN)
		if err != nil {
			_ = hotDB.Close()
			return nil, err
		}
	}

	store := &PostgresArchiveStore{
		hotDB:               hotDB,
		priorityDB:          priorityDB,
		prioritySeverityMax: prioritySeverityMax,
		knownPartitions:     map[string]struct{}{},
	}
	if err := store.init(); err != nil {
		_ = hotDB.Close()
		if priorityDB != hotDB {
			_ = priorityDB.Close()
		}
		return nil, err
	}
	return store, nil
}

func (s *PostgresArchiveStore) Close() error {
	if s.priorityDB != nil && s.priorityDB != s.hotDB {
		if err := s.priorityDB.Close(); err != nil {
			_ = s.hotDB.Close()
			return err
		}
	}
	if s.hotDB != nil {
		return s.hotDB.Close()
	}
	return nil
}

func (s *PostgresArchiveStore) init() error {
	if err := initArchiveTable(s.hotDB, "logs_hot"); err != nil {
		return err
	}
	if err := initArchiveTable(s.priorityDB, "logs_priority"); err != nil {
		return err
	}
	return nil
}

func initArchiveTable(db *sql.DB, tableName string) error {
	quotedTable := quoteIdent(tableName)
	stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL,
			received_at TIMESTAMPTZ NOT NULL,
			source_ip INET NOT NULL,
			hostname TEXT,
			severity SMALLINT,
			facility SMALLINT,
			program TEXT,
			vendor TEXT,
			parsed_ok BOOLEAN NOT NULL DEFAULT FALSE,
			message TEXT NOT NULL,
			raw_message TEXT NOT NULL,
			fortigate_type TEXT,
			fortigate_subtype TEXT,
			fortigate_level TEXT,
			fortigate_action TEXT,
			fortigate_vd TEXT,
			fortigate_srcip INET,
			fortigate_srcport INTEGER,
			fortigate_dstip INET,
			fortigate_dstport INTEGER,
			fortigate_srcintf TEXT,
			fortigate_dstintf TEXT,
			fortigate_policyid BIGINT,
			fortigate_sessionid BIGINT,
			fortigate_proto INTEGER,
			fortigate_service TEXT,
			fortigate_user_name TEXT,
			fortigate_app TEXT,
			fortigate_appcat TEXT,
			PRIMARY KEY (received_at, id)
		) PARTITION BY RANGE (received_at);`, quotedTable)
	if _, err := db.Exec(stmt); err != nil {
		return err
	}

	for _, columnSQL := range []string{
		"ADD COLUMN IF NOT EXISTS fortigate_type TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_subtype TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_level TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_action TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_vd TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_srcip INET",
		"ADD COLUMN IF NOT EXISTS fortigate_srcport INTEGER",
		"ADD COLUMN IF NOT EXISTS fortigate_dstip INET",
		"ADD COLUMN IF NOT EXISTS fortigate_dstport INTEGER",
		"ADD COLUMN IF NOT EXISTS fortigate_srcintf TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_dstintf TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_policyid BIGINT",
		"ADD COLUMN IF NOT EXISTS fortigate_sessionid BIGINT",
		"ADD COLUMN IF NOT EXISTS fortigate_proto INTEGER",
		"ADD COLUMN IF NOT EXISTS fortigate_service TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_user_name TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_app TEXT",
		"ADD COLUMN IF NOT EXISTS fortigate_appcat TEXT",
	} {
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s %s", quotedTable, columnSQL)); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresArchiveStore) Flush(snapshot stats.Snapshot) error {
	if len(snapshot.Events) == 0 {
		return nil
	}

	hotTx, err := s.hotDB.Begin()
	if err != nil {
		return err
	}
	defer hotTx.Rollback()

	hotStmt, err := hotTx.Prepare(archiveInsertSQL("logs_hot"))
	if err != nil {
		return err
	}
	defer hotStmt.Close()

	var priorityTx *sql.Tx
	var priorityStmt *sql.Stmt
	if s.priorityDB != nil {
		if s.priorityDB == s.hotDB {
			priorityTx = hotTx
		} else {
			priorityTx, err = s.priorityDB.Begin()
			if err != nil {
				return err
			}
			defer priorityTx.Rollback()
		}
		priorityStmt, err = priorityTx.Prepare(archiveInsertSQL("logs_priority"))
		if err != nil {
			return err
		}
		defer priorityStmt.Close()
	}

	for _, event := range snapshot.Events {
		date := event.ReceivedAt.UTC().Format("2006_01_02")
		if err := s.ensurePartition(hotTx, "logs_hot", date, event.ReceivedAt.UTC()); err != nil {
			return err
		}
		if _, err := hotStmt.Exec(archiveInsertArgs(event)...); err != nil {
			return err
		}

		if priorityStmt != nil && event.Message.Severity >= 0 && event.Message.Severity <= s.prioritySeverityMax {
			if err := s.ensurePartition(priorityTx, "logs_priority", date, event.ReceivedAt.UTC()); err != nil {
				return err
			}
			if _, err := priorityStmt.Exec(archiveInsertArgs(event)...); err != nil {
				return err
			}
		}
	}

	if err := hotTx.Commit(); err != nil {
		return err
	}
	if priorityTx != nil && priorityTx != hotTx {
		return priorityTx.Commit()
	}
	return nil
}

func (s *PostgresArchiveStore) QueryHotLogs(query ArchiveQuery) ([]ArchivedLogEvent, error) {
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 100
	}
	if query.End.IsZero() {
		query.End = time.Now().UTC()
	}
	if query.Start.IsZero() {
		query.Start = query.End.Add(-1 * time.Hour)
	}

	args := []any{query.Start.UTC(), query.End.UTC()}
	conditions := []string{"received_at >= $1", "received_at <= $2"}
	if query.SourceIP != "" {
		args = append(args, query.SourceIP)
		conditions = append(conditions, fmt.Sprintf("source_ip = $%d", len(args)))
	}
	if query.Hostname != "" {
		args = append(args, query.Hostname)
		conditions = append(conditions, fmt.Sprintf("hostname = $%d", len(args)))
	}
	if query.Severity != nil {
		args = append(args, *query.Severity)
		conditions = append(conditions, fmt.Sprintf("severity = $%d", len(args)))
	}
	if query.Vendor != "" {
		args = append(args, query.Vendor)
		conditions = append(conditions, fmt.Sprintf("vendor = $%d", len(args)))
	}
	args = append(args, query.Limit)

	stmt := fmt.Sprintf(`SELECT
			received_at,
			source_ip::text,
			COALESCE(hostname, ''),
			severity,
			facility,
			COALESCE(program, ''),
			COALESCE(vendor, ''),
			parsed_ok,
			message,
			raw_message,
			COALESCE(fortigate_type, ''),
			COALESCE(fortigate_subtype, ''),
			COALESCE(fortigate_level, ''),
			COALESCE(fortigate_action, ''),
			COALESCE(fortigate_srcip::text, ''),
			COALESCE(fortigate_dstip::text, ''),
			COALESCE(fortigate_user_name, '')
		FROM logs_hot
		WHERE %s
		ORDER BY received_at DESC
		LIMIT $%d`, strings.Join(conditions, " AND "), len(args))

	rows, err := s.hotDB.Query(stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ArchivedLogEvent
	for rows.Next() {
		var event ArchivedLogEvent
		var severity sql.NullInt64
		var facility sql.NullInt64
		if err := rows.Scan(
			&event.ReceivedAt,
			&event.SourceIP,
			&event.Hostname,
			&severity,
			&facility,
			&event.Program,
			&event.Vendor,
			&event.ParsedOK,
			&event.Message,
			&event.RawMessage,
			&event.FortiGateType,
			&event.FortiGateSubtype,
			&event.FortiGateLevel,
			&event.FortiGateAction,
			&event.FortiGateSrcIP,
			&event.FortiGateDstIP,
			&event.FortiGateUserName,
		); err != nil {
			return nil, err
		}
		if severity.Valid {
			value := int(severity.Int64)
			event.Severity = &value
		}
		if facility.Valid {
			value := int(facility.Int64)
			event.Facility = &value
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *PostgresArchiveStore) ApplyRetention(hotDays, priorityDays int) error {
	if err := s.dropOldPartitions(s.hotDB, "logs_hot", hotDays); err != nil {
		return err
	}
	if s.priorityDB != nil {
		return s.dropOldPartitions(s.priorityDB, "logs_priority", priorityDays)
	}
	return nil
}

func (s *PostgresArchiveStore) ensurePartition(tx *sql.Tx, baseTable, dateKey string, ts time.Time) error {
	cacheKey := baseTable + "_" + dateKey
	s.mu.Lock()
	if _, ok := s.knownPartitions[cacheKey]; ok {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	start := ts.UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	partitionName := cacheKey
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
		quoteIdent(partitionName),
		quoteIdent(baseTable),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
	if _, err := tx.Exec(query); err != nil {
		return err
	}

	for _, indexSQL := range []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (source_ip, received_at DESC)`, quoteIdent(partitionName+"_source_ip_ts_idx"), quoteIdent(partitionName)),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (hostname, received_at DESC)`, quoteIdent(partitionName+"_hostname_ts_idx"), quoteIdent(partitionName)),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (severity, received_at DESC)`, quoteIdent(partitionName+"_severity_ts_idx"), quoteIdent(partitionName)),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (vendor, received_at DESC)`, quoteIdent(partitionName+"_vendor_ts_idx"), quoteIdent(partitionName)),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (fortigate_type, received_at DESC)`, quoteIdent(partitionName+"_fg_type_ts_idx"), quoteIdent(partitionName)),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (fortigate_subtype, received_at DESC)`, quoteIdent(partitionName+"_fg_subtype_ts_idx"), quoteIdent(partitionName)),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (fortigate_srcip, received_at DESC)`, quoteIdent(partitionName+"_fg_srcip_ts_idx"), quoteIdent(partitionName)),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (fortigate_dstip, received_at DESC)`, quoteIdent(partitionName+"_fg_dstip_ts_idx"), quoteIdent(partitionName)),
	} {
		if _, err := tx.Exec(indexSQL); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.knownPartitions[cacheKey] = struct{}{}
	s.mu.Unlock()
	return nil
}

func (s *PostgresArchiveStore) dropOldPartitions(db *sql.DB, baseTable string, keepDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -keepDays)
	rows, err := db.Query(`
		SELECT c.relname
		FROM pg_inherits i
		JOIN pg_class p ON i.inhparent = p.oid
		JOIN pg_class c ON i.inhrelid = c.oid
		WHERE p.relname = $1`, baseTable)
	if err != nil {
		return err
	}
	defer rows.Close()

	var toDrop []string
	for rows.Next() {
		var partitionName string
		if err := rows.Scan(&partitionName); err != nil {
			return err
		}
		partitionDate, err := partitionDateFromName(partitionName)
		if err != nil {
			continue
		}
		if partitionDate.Before(cutoff.Truncate(24 * time.Hour)) {
			toDrop = append(toDrop, partitionName)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, partitionName := range toDrop {
		if _, err := db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, quoteIdent(partitionName))); err != nil {
			return err
		}
		s.mu.Lock()
		delete(s.knownPartitions, partitionName)
		s.mu.Unlock()
	}
	return nil
}

func partitionDateFromName(name string) (time.Time, error) {
	if strings.LastIndex(name, "_") < 0 || len(name) < 10 {
		return time.Time{}, fmt.Errorf("invalid partition name")
	}
	return time.Parse("2006_01_02", name[len(name)-10:])
}

func nullableSeverity(value int) any {
	if value < 0 {
		return nil
	}
	return value
}

func archiveInsertSQL(tableName string) string {
	return fmt.Sprintf(`INSERT INTO %s
		(received_at, source_ip, hostname, severity, facility, program, vendor, parsed_ok, message, raw_message,
		 fortigate_type, fortigate_subtype, fortigate_level, fortigate_action, fortigate_vd,
		 fortigate_srcip, fortigate_srcport, fortigate_dstip, fortigate_dstport,
		 fortigate_srcintf, fortigate_dstintf, fortigate_policyid, fortigate_sessionid,
		 fortigate_proto, fortigate_service, fortigate_user_name, fortigate_app, fortigate_appcat)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28)`, quoteIdent(tableName))
}

func archiveInsertArgs(event stats.Event) []any {
	fg := event.Message.FortiGate
	return []any{
		event.ReceivedAt.UTC(),
		event.SourceIP,
		nullableString(event.Message.Hostname),
		nullableSeverity(event.Message.Severity),
		nullableSeverity(event.Message.Facility),
		nullableString(event.Message.Program),
		event.Message.Vendor,
		event.Message.ParsedOK,
		event.Message.RawMessage,
		event.Message.RawMessage,
		nullableString(fg.Type),
		nullableString(fg.Subtype),
		nullableString(fg.Level),
		nullableString(fg.Action),
		nullableString(fg.VD),
		nullableString(fg.SrcIP),
		nullableInt(fg.SrcPort),
		nullableString(fg.DstIP),
		nullableInt(fg.DstPort),
		nullableString(fg.SrcIntf),
		nullableString(fg.DstIntf),
		nullableInt64(fg.PolicyID),
		nullableInt64(fg.SessionID),
		nullableInt(fg.Proto),
		nullableString(fg.Service),
		nullableString(fg.UserName),
		nullableString(fg.App),
		nullableString(fg.AppCat),
	}
}

func nullableString(value string) any {
	if value == "" || value == "unknown" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
