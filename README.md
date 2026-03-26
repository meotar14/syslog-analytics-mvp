# Syslog Analytics MVP

Minimal analytic syslog collector for capacity sizing.

Current release line: `v0.1.x`

## What it does

- accepts syslog over UDP and TCP
- stores only aggregated counters in SQLite
- tracks per-minute traffic, byte volume, source spread, severity and facility
- exposes a small dashboard and JSON API

## What it does not do

- it does not keep raw log messages
- it does not support search over log content
- it is not intended for forensic retention

## Local development

Requirements:

- Go 1.22+
- Docker, if you want to build an image

Run locally:

```bash
go mod download
go run ./cmd/syslog-analytics
```

Open:

- dashboard: `http://localhost:8080`
- UDP syslog: `localhost:5514/udp`
- TCP syslog: `localhost:5514/tcp`

Example test message:

```bash
echo '<34>Oct 11 22:14:15 firewall01 sshd[123]: Failed password for admin' | nc -u -w1 127.0.0.1 5514
```

## API

- `GET /api/overview?range_minutes=1440`
- `GET /api/timeseries?range_minutes=1440`
- `GET /api/sources?range_minutes=1440&limit=10`
- `GET /api/severity?range_minutes=1440`
- `GET /api/facility?range_minutes=1440`
- `GET /api/health`
- `GET /api/settings`
- `POST /api/settings`

## Storage model

- `stats_second`
- `stats_minute`
- `stats_hour`
- `stats_day`
- `stats_by_source_minute`
- `stats_by_severity_minute`
- `stats_by_facility_minute`
- `source_registry`

## Implemented from the initial plan

- single-container MVP for ingest, aggregation, storage and dashboard
- UDP syslog listener on `5514/udp`
- TCP syslog listener on `5514/tcp`
- lightweight parser extracting hostname, program, severity and facility when present
- basic support for RFC3164 and RFC5424 style headers
- aggregated storage only, no raw log retention
- second, minute, hour and day traffic buckets
- source, severity and facility rollups per minute
- SQLite persistence with retention cleanup
- embedded HTTP dashboard with overview, trend, top sources and dimension breakdowns
- retention settings editable from the dashboard and persisted in SQLite
- automatic dashboard refresh via lightweight polling every 5 seconds
- GitHub Actions build and publish to GHCR
- container build metadata exposed through `GET /api/health`

## Still planned

- better RFC3164 and RFC5424 parsing
- CSV export
- alerting on bursts, parse failures or source anomalies

## Deployment

Pull and run with the published container image:

```bash
docker compose pull
docker compose up -d
```

Default published image:

- `ghcr.io/meotar14/syslog-analytics:latest`

Pin a specific release:

```bash
export SYSLOG_ANALYTICS_TAG=v0.1.2
docker compose up -d
```

## Versioning

- release tags will use `vMAJOR.MINOR.PATCH`
- `latest` follows the default branch
- the app exposes version, commit and build date in `/api/health`

## Changelog

See [`CHANGELOG.md`](./CHANGELOG.md).

## Next steps for the project
- create a proper GitHub release object for the latest tag
- stabilize schema and config before adding exports and alerting
