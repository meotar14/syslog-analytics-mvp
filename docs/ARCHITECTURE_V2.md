# Syslog Archive Architecture v2

This document describes the recommended next phase after the SQLite sizing MVP.

## Summary

Use `PostgreSQL` as the primary raw-log database and keep the current MVP as a sizing and health view.

Target retention:

- all logs: `30 days`
- high-severity logs: `365 days`

Recommended severity retention rule:

- keep everything in the hot path for 30 days
- duplicate `severity 0-3` into a long-retention table for 365 days

This keeps the implementation simple and operationally predictable.

## Why PostgreSQL

Based on the measured workload:

- roughly `7.8M` messages per 7 days
- roughly `6.6 GB` raw payload per 7 days
- average ingest around `13 msg/s`
- peak around `58 msg/s`

That workload is well within normal PostgreSQL territory if the schema is partitioned by date and queries are time-bounded.

PostgreSQL is the recommended choice because it provides:

- low operational complexity
- good structured filtering
- partitioning by day
- straightforward retention by dropping partitions
- room for vendor-specific extracted fields

## Recommended Architecture

### 1. Ingest

Two acceptable options:

- direct Go listener over `UDP/TCP syslog`
- `syslog-ng` or `rsyslog` fronting a Go parser service

For this project, the pragmatic path is:

- keep the Go service as the receiver
- improve parsing for RFC3164, RFC5424 and FortiGate key-value logs

### 2. Processing

The parser layer should produce:

- normalized envelope fields
- vendor-specific extracted fields
- a `parsed_ok` flag
- a `severity_bucket` or `is_priority` flag for retention routing

Core normalized fields:

- `received_at`
- `source_ip`
- `hostname`
- `severity`
- `facility`
- `program`
- `message`
- `raw_message`
- `vendor`
- `parsed_ok`

FortiGate extracted fields to prioritize:

- `date`
- `time`
- `devid`
- `devname`
- `type`
- `subtype`
- `level`
- `vd`
- `srcip`
- `srcport`
- `dstip`
- `dstport`
- `srcintf`
- `dstintf`
- `action`
- `policyid`
- `sessionid`
- `proto`
- `service`
- `user`
- `app`
- `appcat`

### 3. Storage

Use three logical storage layers:

1. `logs_hot`
2. `logs_priority`
3. `log_stats_hourly`

`logs_hot`

- all messages
- 30-day retention
- partitioned by day

`logs_priority`

- only high-severity messages
- 365-day retention
- partitioned by day

`log_stats_hourly`

- dashboard aggregations
- much longer retention allowed

### 4. UI / Query API

Expose separate query paths:

- recent search in `logs_hot`
- long-range high-severity search in `logs_priority`
- dashboard reads from `log_stats_hourly`

Do not drive dashboards directly from raw message tables if it can be avoided.

## Retention Strategy

Do not run large `DELETE` jobs.

Use daily partitions and drop partitions instead:

- `logs_hot_YYYY_MM_DD`
- `logs_priority_YYYY_MM_DD`

Daily retention jobs:

- drop `logs_hot` partitions older than 30 days
- drop `logs_priority` partitions older than 365 days

This is materially cheaper and safer than row-based cleanup.

The runtime implementation creates daily partitions as messages arrive. Operator retention settings map directly to the number of days kept for each partition family:

- `ARCHIVE_HOT_RETENTION_DAYS` controls `logs_hot_*`
- `ARCHIVE_PRIORITY_RETENTION_DAYS` controls `logs_priority_*`
- `ARCHIVE_PRIORITY_SEVERITY_MAX` controls which severities are duplicated into the priority archive

Changing retention affects the next maintenance cycle. Lowering a value can drop many old partitions at once, so operators should treat large reductions as operational maintenance changes.

## Query Model

Primary search filters:

- time range
- source IP
- hostname
- severity
- facility
- vendor
- FortiGate `type`
- FortiGate `subtype`
- FortiGate `action`
- FortiGate `srcip`
- FortiGate `dstip`
- FortiGate `user`

Avoid broad full-text search as the default interaction.
Time-bounded structured search should be the primary UX.

## Parse Failure Problem

The current sizing console shows a very high parse-failure ratio. That is not a database problem.

The likely root cause is that FortiGate messages are mostly key-value formatted payloads and the current parser is too generic.

The next parser milestone should be:

1. detect FortiGate payloads
2. parse key-value pairs
3. map FortiGate `level` to normalized severity if needed
4. persist both normalized fields and the raw original line

## Growth Path

Stay on PostgreSQL until one of these becomes true:

- sustained ingest is much higher than current measurements
- you need very large-scale analytics over many months of raw logs
- you need complex aggregations over hundreds of millions of rows

If that happens later:

- keep PostgreSQL for operational search
- add ClickHouse for heavy historical analytics

That is a future split, not the current recommendation.

## Recommended Next Milestones

1. Add PostgreSQL storage layer beside the current SQLite sizing layer
2. Implement `logs_hot` and `logs_priority` partitioned tables
3. Add FortiGate key-value parser
4. Add recent-log UI with structured filters
5. Add partition maintenance jobs
6. Add hourly aggregation jobs for dashboard reads
