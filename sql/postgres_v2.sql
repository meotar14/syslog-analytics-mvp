CREATE TABLE IF NOT EXISTS logs_hot (
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
) PARTITION BY RANGE (received_at);

CREATE TABLE IF NOT EXISTS logs_priority (
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
) PARTITION BY RANGE (received_at);

CREATE TABLE IF NOT EXISTS log_stats_hourly (
    hour_ts TIMESTAMPTZ NOT NULL,
    source_ip INET,
    hostname TEXT,
    severity SMALLINT,
    facility SMALLINT,
    vendor TEXT,
    msg_count BIGINT NOT NULL,
    byte_count BIGINT NOT NULL,
    parsed_fail_count BIGINT NOT NULL,
    PRIMARY KEY (hour_ts, source_ip, hostname, severity, facility, vendor)
);

CREATE INDEX IF NOT EXISTS idx_log_stats_hourly_hour_ts
    ON log_stats_hourly (hour_ts DESC);

CREATE OR REPLACE FUNCTION create_daily_log_partition(base_table TEXT, target_date DATE)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    partition_name TEXT;
    range_start TIMESTAMPTZ;
    range_end TIMESTAMPTZ;
BEGIN
    partition_name := format('%s_%s', base_table, to_char(target_date, 'YYYY_MM_DD'));
    range_start := target_date::timestamptz;
    range_end := (target_date + INTERVAL '1 day')::timestamptz;

    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
        partition_name,
        base_table,
        range_start,
        range_end
    );

    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (source_ip, received_at DESC)', partition_name || '_source_ip_ts_idx', partition_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (hostname, received_at DESC)', partition_name || '_hostname_ts_idx', partition_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (severity, received_at DESC)', partition_name || '_severity_ts_idx', partition_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (vendor, received_at DESC)', partition_name || '_vendor_ts_idx', partition_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (fortigate_type, received_at DESC)', partition_name || '_fg_type_ts_idx', partition_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (fortigate_subtype, received_at DESC)', partition_name || '_fg_subtype_ts_idx', partition_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (fortigate_srcip, received_at DESC)', partition_name || '_fg_srcip_ts_idx', partition_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I (fortigate_dstip, received_at DESC)', partition_name || '_fg_dstip_ts_idx', partition_name);
END;
$$;

CREATE OR REPLACE FUNCTION drop_old_log_partitions(base_table TEXT, keep_days INTEGER)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    cutoff DATE;
    partition_record RECORD;
    partition_date DATE;
BEGIN
    cutoff := CURRENT_DATE - keep_days;

    FOR partition_record IN
        SELECT c.relname AS partition_name
        FROM pg_inherits i
        JOIN pg_class p ON i.inhparent = p.oid
        JOIN pg_class c ON i.inhrelid = c.oid
        WHERE p.relname = base_table
    LOOP
        BEGIN
            partition_date := to_date(right(partition_record.partition_name, 10), 'YYYY_MM_DD');
            IF partition_date < cutoff THEN
                EXECUTE format('DROP TABLE IF EXISTS %I', partition_record.partition_name);
            END IF;
        EXCEPTION WHEN OTHERS THEN
            NULL;
        END;
    END LOOP;
END;
$$;
