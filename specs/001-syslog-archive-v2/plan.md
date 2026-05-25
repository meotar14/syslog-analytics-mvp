# Implementation Plan: Syslog Archive v2

**Branch**: `[001-syslog-archive-v2]` | **Date**: 2026-05-22 | **Spec**: [`specs/001-syslog-archive-v2/spec.md`](/Users/liscakl/Library/Mobile%20Documents/com~apple~CloudDocs/Projects/Syslog/syslog-analytics-mvp/specs/001-syslog-archive-v2/spec.md)  
**Input**: Feature specification from `/specs/001-syslog-archive-v2/spec.md`

## Summary

Evolve the current SQLite analytics MVP into a dual-mode system:

- preserve the standalone analytics collector
- add PostgreSQL-backed raw archive sinks for hot and priority retention
- introduce structured raw-log archive retrieval
- prioritize FortiGate parsing to improve data usability

The implementation should keep the current dashboard alive while incrementally adding archive functionality behind explicit configuration.

## Technical Context

**Language/Version**: Go 1.22  
**Primary Dependencies**: `modernc.org/sqlite`, `github.com/jackc/pgx/v5`  
**Storage**: SQLite for analytics mode; PostgreSQL for raw archive mode  
**Testing**: Go tests when toolchain is available; integration checks via Docker Compose and health/query endpoints  
**Target Platform**: Linux server / Docker deployment  
**Project Type**: backend service with embedded web dashboard  
**Performance Goals**: comfortably handle the measured workload around 13 msg/s average and roughly 58 msg/s peak while preserving room for operational growth  
**Constraints**: operator-controlled retention, split storage placement for hot vs priority archive, mobile-usable dashboard, Docker-first deployment  
**Scale/Scope**: millions of logs per week, 30-day hot archive, 365-day priority archive, FortiGate traffic as primary real-world source

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Dual-Product Separation**: Pass, because the plan preserves analytics-only mode and adds archive mode beside it.
- **Structured Logs First**: Pass, because archive work prioritizes normalized fields and FortiGate parsing.
- **Retention Must Be Operationally Adjustable**: Pass, because hot and priority retention remain configurable and partition-driven.
- **Storage Placement Must Be Explicit**: Pass, because hot and priority DSN are independently configurable.
- **Observability and Safety Over Cleverness**: Pass, because the rollout stays incremental and health/config visibility remains part of the design.

## Project Structure

### Documentation (this feature)

```text
specs/001-syslog-archive-v2/
├── plan.md
└── spec.md
```

### Source Code (repository root)

```text
cmd/
└── syslog-analytics/
    └── main.go

internal/
├── api/
│   ├── http.go
│   └── index.html
├── config/
│   └── config.go
├── ingest/
│   ├── tcp.go
│   └── udp.go
├── parse/
│   └── syslog.go
├── settings/
│   └── runtime.go
├── stats/
│   └── stats.go
└── storage/
    ├── sqlite.go
    └── postgres_archive.go

docs/
└── ARCHITECTURE_V2.md

sql/
└── postgres_v2.sql

docker-compose.postgres.yml
```

**Structure Decision**: Keep the existing single-service repository structure and evolve it with dual storage backends. Do not split into separate repositories yet; preserve shared ingest/parser/dashboard code while keeping analytics-only and archive-enabled deployment modes distinct through configuration.

## Phase Breakdown

### Phase 1 - Backend Topology Foundation

Goal:

- keep SQLite analytics intact
- make PostgreSQL archive sinks real and configurable

Work:

- finalize hot vs priority archive DSN handling
- ensure archive health reporting is reliable
- verify archive retention settings are represented clearly
- normalize archive configuration terminology across code and docs

### Phase 2 - Archive Data Quality

Goal:

- make archived FortiGate traffic actually useful to query

Work:

- detect FortiGate payloads
- parse key-value pairs
- retain raw original line alongside extracted fields
- improve parse status visibility

### Phase 3 - Archive Query Surface

Goal:

- provide bounded raw-log retrieval without requiring direct SQL access

Work:

- add archive query endpoints
- support filtering by time range, source IP, hostname, severity and vendor
- add minimal pagination / bounded result windows
- expose archive mode status in health/output surfaces

### Phase 4 - Integrated Analytics + Archive UX

Goal:

- keep analytics dashboards while making archive mode usable from the same product

Work:

- preserve current dashboard
- add archive search entry points
- keep analytics-only deployment functional
- keep operator-facing retention controls understandable

## Recommended Implementation Order

1. Stabilize the PostgreSQL archive sink already introduced in the worktree.
2. Add FortiGate parsing and extracted archive fields.
3. Add archive read/query endpoints.
4. Add minimal archive browsing UI.
5. Reconcile runtime settings so archive retention and analytics retention are exposed cleanly.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Dual storage backends in one service | Needed to preserve analytics-only mode while adding archive mode incrementally | A full rewrite into a new service would slow down delivery and duplicate working ingest/dashboard logic |
| Separate hot and priority storage targets | Needed for operator-controlled placement on different disks or DB instances | A single storage target cannot satisfy the explicit requirement to route short-term and long-term archives independently |
