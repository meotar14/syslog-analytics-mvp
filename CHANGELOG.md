# Changelog

## v0.1.4 - 2026-03-26

- switched the dashboard to a dark visual theme
- added `meotar` branding and build/release badge in the header
- clarified branch-build vs release-tag display in the UI

## v0.1.3 - 2026-03-26

- added automatic dashboard refresh with 5 second polling
- improved average rate display for low-traffic environments
- added average messages per minute card to the dashboard

## v0.1.2 - 2026-03-26

- added runtime retention settings API backed by SQLite
- added retention settings panel to the dashboard
- cleanup logic now uses persisted runtime settings instead of only startup environment values

## v0.1.1 - 2026-03-26

- added TCP syslog ingest alongside UDP
- added support for octet-counted and newline-delimited TCP framing
- improved parser coverage for common RFC3164 and RFC5424 style headers
- added TCP listener visibility to the health endpoint and dashboard
- updated deployment compose to expose TCP syslog input

## v0.1.0 - 2026-03-26

- initial public MVP release
- UDP syslog ingest with basic metadata parsing
- aggregated SQLite storage without raw message retention
- dashboard for overview, traffic trend, top sources, severity and facility
- GHCR image publishing workflow
- compose deployment updated to pull published images
- build metadata added to the binary and `/api/health`
