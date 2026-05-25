# Feature Specification: Syslog Archive v2

**Feature Branch**: `[001-syslog-archive-v2]`  
**Created**: 2026-05-22  
**Status**: Draft  
**Input**: User description: "Vybudovať plnohodnotný syslog archive server nad existujúcim analytickým projektom, s dynamickou retenciou, integrovanou analytikou, samostatným analytics režimom a oddeleným hot/priority archivom, vrátane možnosti mapovať krátkodobý a dlhodobý archív na rozdielne úložiská."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Archivovať a prehliadať logy (Priority: P1)

Ako administrátor chcem, aby systém prijímal syslog správy priamo zo zariadení, ukladal ich do raw archívu a umožňoval ich filtrované prehliadanie podľa času, zdroja a závažnosti.

**Why this priority**: Toto je jadro cieľa. Bez raw archívu a prehliadania nejde o produkčný syslog server.

**Independent Test**: Dá sa samostatne otestovať zaslaním UDP/TCP syslog správ, overením zápisu do archive storage a dotazom cez archive API na časovo ohraničené logy.

**Acceptance Scenarios**:

1. **Given** že zariadenie posiela syslog správy do archive servera, **When** správy prídu cez UDP alebo TCP, **Then** systém ich uloží do krátkodobého archívu a sprístupní ich cez API na prehliadanie.
2. **Given** že operátor filtruje podľa času, source IP a severity, **When** vykoná archive query, **Then** dostane len zodpovedajúce raw logy v stabilnom formáte.

---

### User Story 2 - Dynamicky riadiť retenciu (Priority: P1)

Ako administrátor chcem meniť dĺžku uchovávania krátkodobého aj dlhodobého archívu bez zásahu do kódu, aby som vedel reagovať na dostupný priestor alebo politiku uchovávania.

**Why this priority**: Retencia je explicitne požadovaná a je kľúčová pre prevádzku, diskový priestor aj auditné požiadavky.

**Independent Test**: Dá sa samostatne otestovať zmenou retention nastavení, následným maintenance cyklom a overením, že staršie partitiony sa vyraďujú podľa novej politiky.

**Acceptance Scenarios**:

1. **Given** že hot archive retention je nastavená na 30 dní, **When** operátor ju zníži na 14 dní, **Then** systém použije novú hodnotu pri ďalšom retention cykle.
2. **Given** že priority retention je nastavená na 365 dní, **When** operátor ju zmení na 90 dní, **Then** dlhodobý archív sa čistí podľa novej hodnoty bez potreby meniť aplikáciu ručne v kóde.

---

### User Story 3 - Integrovaná analytika v archive serveri (Priority: P2)

Ako administrátor chcem mať v archive serveri aj analytics dashboard podobný dnešnému sizing nástroju, aby som nemusel prevádzkovať dva úplne odlišné UI na základný prehľad o trafficu.

**Why this priority**: Analytics nie sú primárny archivačný cieľ, ale sú dôležité pre prevádzkovú viditeľnosť a už boli používateľom explicitne požadované.

**Independent Test**: Dá sa samostatne otestovať tým, že archive server prijíma logy a dashboard nad nimi zobrazuje throughput, top sources a severity/facility mix.

**Acceptance Scenarios**:

1. **Given** že archive server prijíma logy, **When** operátor otvorí dashboard, **Then** vidí agregované štatistiky podobné existujúcemu analytics módu.

---

### User Story 4 - Samostatný analytics produkt (Priority: P3)

Ako administrátor chcem zachovať možnosť prevádzkovať analytics collector samostatne, aby som ho vedel použiť na sizing alebo dočasný monitoring aj bez raw archívu.

**Why this priority**: Toto je strategická požiadavka na zachovanie dvoch režimov použitia, nie blokujúca podmienka pre archive MVP.

**Independent Test**: Dá sa samostatne otestovať spustením analytics-only režimu bez PostgreSQL a overením, že dashboard a agregácie fungujú bez raw-log storage.

**Acceptance Scenarios**:

1. **Given** že operátor nasadí analytics-only variant, **When** pošle syslog traffic, **Then** sizing dashboard funguje bez požiadavky na PostgreSQL archive backend.

## Edge Cases

- Čo sa stane, keď hot archive DB funguje, ale priority archive DB je nedostupná?
- Ako sa systém správa, keď retention hodnoty klesnú tak nízko, že ďalší maintenance cyklus odstráni veľké množstvo partitionov?
- Ako sa správa archive query pri neparsovaných FortiGate správach, ktoré zatiaľ nemajú vendor-specific extrahované polia?
- Čo sa stane, keď short-term a long-term archive smerujú na rovnaký PostgreSQL server?
- Ako sa systém zachová pri TCP framing rozdieloch medzi newline-delimited a octet-counted správami?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support direct syslog ingest over UDP and TCP for the archive product.
- **FR-002**: System MUST persist all accepted raw log events into a short-term hot archive store.
- **FR-003**: System MUST duplicate high-priority log events into a long-term priority archive store based on a configurable severity threshold.
- **FR-004**: System MUST allow operators to configure hot archive retention days.
- **FR-005**: System MUST allow operators to configure priority archive retention days.
- **FR-006**: System MUST allow operators to configure the severity threshold that defines which events are duplicated into the long-term archive.
- **FR-007**: System MUST support independent storage targets for hot and priority archives.
- **FR-008**: System MUST retain the current analytics dashboard capability in the archive product.
- **FR-009**: System MUST preserve a deployable analytics-only mode that does not require raw archive storage.
- **FR-010**: System MUST store the raw original log line for each archived event.
- **FR-011**: System MUST support archive browsing via structured filters at minimum by time range, source IP, hostname, severity and vendor.
- **FR-012**: System MUST expose backend health that indicates whether analytics storage, hot archive storage and priority archive storage are enabled.
- **FR-013**: System MUST implement archive retention using partition lifecycle management rather than large row-based cleanup jobs.
- **FR-014**: System MUST support FortiGate-oriented structured parsing as a priority vendor path for archive usefulness.

### Key Entities *(include if feature involves data)*

- **ArchivedLogEvent**: Raw syslog event with normalized envelope fields and optional vendor-specific extracted fields.
- **HotArchiveRetentionPolicy**: Operator-managed rule that defines how long all logs remain in the hot archive.
- **PriorityArchiveRetentionPolicy**: Operator-managed rule that defines how long high-priority logs remain in the long-term archive.
- **ArchiveStorageTarget**: Connection and placement definition for hot or priority archive storage.
- **AnalyticsAggregate**: Time-bucketed counters used by the dashboard and independent sizing mode.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can ingest and retrieve archived syslog events for a bounded time range without manually inspecting raw database tables.
- **SC-002**: Operators can reduce or increase hot and priority retention windows through configuration without changing application code.
- **SC-003**: The system can run in one of two supported deployment modes: analytics-only and archive+analytics.
- **SC-004**: Operators can deploy hot and priority archives onto separate storage targets and observe the configured topology through health/status endpoints.
- **SC-005**: For FortiGate traffic, parse quality improves enough that structured fields become useful for archive filtering rather than leaving almost all events as generic raw lines.

## Assumptions

- PostgreSQL remains the primary archive backend for the near-term implementation.
- The existing SQLite-backed analytics path remains acceptable for the standalone analytics collector.
- The first archive browsing release can start with API and minimal filtered retrieval before a full polished raw-log UI is complete.
- Disk placement for hot and priority storage is primarily controlled through deployment configuration rather than end-user UI controls.
