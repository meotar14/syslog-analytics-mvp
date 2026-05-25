# Syslog Analytics / Archive Constitution

## Core Principles

### I. Dual-Product Separation
The repository MUST preserve a clear separation between:

- the standalone analytics collector
- the full syslog archive server

Shared code is encouraged, but analytics-only behavior MUST remain runnable without requiring PostgreSQL raw-log storage.

### II. Structured Logs First
The archive product MUST prefer structured parsing and indexed fields over raw-text-only storage.

At minimum, each persisted log path MUST retain:

- received timestamp
- source IP
- severity
- facility
- hostname when available
- raw original message

Vendor-specific extraction, especially for FortiGate, is a first-class requirement rather than a nice-to-have.

### III. Retention Must Be Operationally Adjustable
Retention rules MUST be designed for operator control.

This includes:

- short-term hot archive retention
- long-term priority archive retention
- configurable severity threshold for priority duplication

Large retention changes MUST be implementable through partition lifecycle management, not massive row deletes.

### IV. Storage Placement Must Be Explicit
Hot archive and long-term priority archive MUST be independently placeable on different storage targets.

The system MUST support:

- same-database deployment for simple installs
- split-database deployment for different disks or different PostgreSQL instances

Infrastructure-level storage routing is part of the product design, not an afterthought.

### V. Observability and Safety Over Cleverness
The system MUST expose enough health and configuration state to be operationally debuggable.

New features should favor:

- explicit configuration
- understandable failure modes
- measurable ingest and retention behavior

Avoid unnecessary complexity such as multi-hop relay chains if direct ingest into the target system is feasible.

## Technology and Deployment Constraints

- The current implementation language is Go.
- SQLite remains acceptable for the standalone analytics collector.
- PostgreSQL is the default and preferred raw archive backend.
- Web UI changes must remain usable on desktop and mobile.
- Features that materially affect ingest, retention, or storage layout must remain deployable in Docker-based environments.

## Development Workflow and Quality Gates

- New archive features should be introduced without breaking the existing analytics dashboard path unless an intentional migration is documented.
- Parser work for FortiGate should precede heavy UI investment in raw-log browsing, because data quality is more critical than presentation polish.
- Any storage design change must document:
  - retention behavior
  - disk placement implications
  - migration impact
- Health endpoints and documentation must be updated when backend topology changes.

## Governance

This constitution governs both the analytics MVP and the archive evolution path in this repository.

Any amendment that weakens:

- dual-product separation
- operator-controlled retention
- storage placement flexibility
- structured parsing

must be explicitly justified in architecture documentation before implementation proceeds.

**Version**: 1.0.0 | **Ratified**: 2026-05-22 | **Last Amended**: 2026-05-22
