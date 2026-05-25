# Tasks: Syslog Archive v2

**Input**: Design documents from `/specs/001-syslog-archive-v2/`
**Prerequisites**: `plan.md`, `spec.md`

**Tests**: Add focused Go tests for parser and archive query behavior where the local Go toolchain is available.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it touches different files or has no dependency on the sibling task.
- **[Story]**: Maps the task to the user story from `spec.md`.

## Phase 1: Setup and Workflow Alignment

**Purpose**: Put the feature back into a valid Spec Kit workflow and document the existing architecture baseline.

- [x] T001 Create and use feature branch `001-syslog-archive-v2`
- [x] T002 [P] Add project constitution in `.specify/memory/constitution.md`
- [x] T003 [P] Add feature specification in `specs/001-syslog-archive-v2/spec.md`
- [x] T004 [P] Add implementation plan in `specs/001-syslog-archive-v2/plan.md`
- [x] T005 [P] Document archive architecture in `docs/ARCHITECTURE_V2.md`
- [x] T006 [P] Add PostgreSQL deployment skeleton in `docker-compose.postgres.yml`
- [x] T007 [P] Add PostgreSQL schema design in `sql/postgres_v2.sql`

---

## Phase 2: Foundational Archive Topology

**Purpose**: Core storage and configuration work that blocks archive user stories.

- [x] T008 Add archive configuration fields in `internal/config/config.go`
- [x] T009 Add PostgreSQL dependency in `go.mod`
- [x] T010 Add event buffering to analytics snapshots in `internal/stats/stats.go`
- [x] T011 Implement initial PostgreSQL archive sink in `internal/storage/postgres_archive.go`
- [x] T012 Wire archive sink flush and retention into `cmd/syslog-analytics/main.go`
- [x] T013 Expose archive backend status in `internal/api/http.go`
- [x] T014 Align runtime-created PostgreSQL schema with `sql/postgres_v2.sql`
- [ ] T015 Add basic archive store tests in `internal/storage/postgres_archive_test.go`

**Checkpoint**: Analytics-only mode still runs without PostgreSQL; archive mode can write raw events when DSNs are configured.

---

## Phase 3: User Story 1 - Archive and Browse Logs (Priority: P1)

**Goal**: Ingest syslog over UDP/TCP, persist raw events to hot archive storage, and retrieve them with bounded structured filters.

**Independent Test**: Send UDP/TCP syslog messages, verify inserts in `logs_hot`, then query via archive API by time range, source IP and severity.

- [x] T016 [US1] Define archive query types in `internal/storage/postgres_archive.go`
- [x] T017 [US1] Implement bounded hot archive query method in `internal/storage/postgres_archive.go`
- [x] T018 [US1] Wire archive store into HTTP server construction in `internal/api/http.go` and `cmd/syslog-analytics/main.go`
- [x] T019 [US1] Add `GET /api/archive/logs` with filters for time range, source IP, hostname, severity, vendor and limit in `internal/api/http.go`
- [x] T020 [US1] Add archive query validation and defensive default limit in `internal/api/http.go`
- [x] T021 [P] [US1] Add API documentation to `README.md`

**Checkpoint**: User Story 1 works independently through API without direct database inspection.

---

## Phase 4: User Story 2 - Dynamic Retention (Priority: P1)

**Goal**: Make hot and priority archive retention configurable and partition-driven.

**Independent Test**: Create old partitions, change retention days, run maintenance and verify only partitions outside the retention window are dropped.

- [x] T022 [US2] Harden partition name quoting and retention partition drop logic in `internal/storage/postgres_archive.go`
- [x] T023 [US2] Include hot and priority retention values in `/api/health` in `internal/api/http.go`
- [x] T024 [US2] Document archive retention environment variables and examples in `README.md`
- [x] T025 [P] [US2] Add retention behavior notes to `docs/ARCHITECTURE_V2.md`

**Checkpoint**: Retention changes are visible operationally and executed through partition lifecycle management.

---

## Phase 5: User Story 3 - Integrated Archive Analytics (Priority: P2)

**Goal**: Preserve the existing analytics dashboard while archive mode is enabled.

**Independent Test**: Run archive+analytics mode, ingest messages, confirm dashboard APIs still update and archive health reports enabled storage.

- [ ] T026 [US3] Verify analytics flush remains independent from archive flush failures in `cmd/syslog-analytics/main.go`
- [ ] T027 [US3] Add archive status and retention values to the dashboard data path in `internal/api/http.go`
- [ ] T028 [US3] Add minimal archive search entry point to `internal/api/index.html`

**Checkpoint**: Archive mode adds raw-log capabilities without removing the sizing dashboard.

---

## Phase 6: User Story 4 - Analytics-Only Product (Priority: P3)

**Goal**: Keep the standalone analytics collector deployable without PostgreSQL.

**Independent Test**: Start without archive DSNs, send syslog traffic, verify dashboard and settings API work and `/api/health` reports archive disabled.

- [x] T029 [US4] Ensure archive routes fail clearly or report disabled when no PostgreSQL DSN is configured in `internal/api/http.go`
- [x] T030 [P] [US4] Document analytics-only deployment in `README.md`
- [ ] T031 [US4] Keep existing `docker-compose.yml` free of mandatory PostgreSQL dependencies

**Checkpoint**: Analytics-only and archive+analytics remain two supported modes.

---

## Phase 7: FortiGate Archive Data Quality

**Purpose**: Improve archive usefulness by extracting FortiGate structured fields before heavier UI work.

- [x] T032 [US1] Add FortiGate key-value fields to `parse.Message` in `internal/parse/syslog.go`
- [x] T033 [US1] Implement FortiGate payload detection and key-value parsing in `internal/parse/syslog.go`
- [x] T034 [US1] Map FortiGate `level` to normalized severity when PRI severity is absent in `internal/parse/syslog.go`
- [x] T035 [US1] Persist FortiGate extracted fields in `internal/storage/postgres_archive.go`
- [x] T036 [P] [US1] Add FortiGate parser tests in `internal/parse/syslog_test.go`

---

## Phase 8: Polish and Verification

**Purpose**: Cross-cutting documentation, checks and cleanup.

- [ ] T037 [P] Update `specs/001-syslog-archive-v2/plan.md` if implementation order changes
- [ ] T038 Run `gofmt` on changed Go files
- [ ] T039 Run `go test ./...` when Go is available locally
- [x] T040 Validate Spec Kit prerequisites with `.specify/scripts/bash/check-prerequisites.sh --json`

## Dependencies and Execution Order

- Phase 1 is complete and establishes workflow context.
- Phase 2 mostly exists in the worktree and should be finished before query/API work.
- User Stories 1 and 2 are both P1. Start with US1 query API after schema alignment, then harden retention.
- FortiGate parsing can be implemented before or alongside the US1 query API because it improves archive filter quality.
- User Stories 3 and 4 must remain regression checks throughout all phases.
