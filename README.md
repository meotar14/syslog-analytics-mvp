# Syslog Analytics MVP

Minimal analytic syslog collector for capacity sizing.

## What it does

- accepts syslog over UDP
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

## Storage model

- `stats_second`
- `stats_minute`
- `stats_hour`
- `stats_day`
- `stats_by_source_minute`
- `stats_by_severity_minute`
- `stats_by_facility_minute`
- `source_registry`

## Next steps

- add TCP syslog ingest
- expose retention settings in UI
- improve parser for RFC3164 and RFC5424
- add CSV export and alert thresholds
