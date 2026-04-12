# Release Notes

## Unreleased

### Added

- `chli parl department [--historic]` — List federal departments (EDA, EDI, EJPD, VBS, EFD, WBF, UVEK, BK, …). Falls back to the legacy `ws-old.parlament.ch` endpoint because the current OData service at `ws.parlament.ch` does not expose departments. `--historic` includes end-dated records with `From`/`To` dates.
- `chli parl events [--sessions] [--category <c>] [--limit N] [--all]` — Upcoming parliamentary events from the parlament.ch agenda (sessions, press conferences, ceremonies). Uses the site's anonymous SharePoint Search endpoint, giving structured data (start/end, localized title, category, location) further out than the OData Session entity publishes. Respects `--lang de|fr|it|en` for localized titles and categories.
- `chli parl` (root) — When invoked without a subcommand, prints the current session (or the most recent past + next upcoming session) before the help text. Falls back to the agenda search for the next session when OData hasn't registered it yet.

### Changed

- `chli parl session` — List now includes future sessions announced on the parlament.ch agenda but not yet registered in OData (merged on top, deduped by start date), and adds `Name` (human-readable session name, e.g. "Frühjahrssession 2026") and `Status` (past/current/upcoming) columns. Agenda header and summary table also surface `SessionName`.
- `api/parl_types.go`: `ParlSession` gained `SessionName`, `SessionNumber`, and `TypeName` fields — previously the `Title` column was shown but `SessionName` (the actual human-readable title) was not fetched.
- `api/client.go`: `do()` no longer unconditionally overwrites a pre-set `User-Agent`. Callers that need a non-Chrome UA (e.g. the Akamai-fronted `ws-old.parlament.ch`) can now set one via `DoGetWithHeaders`.

## v0.1.0 — Initial Release

First public release of chli, a unified CLI for Swiss federal open data.

### What's Included

**Five data sources in one binary:**
- **Swiss Parliament** — Members, votes, business items, committees via OData v3
- **Federal Law (Fedlex)** — SR lookups, full-text search, Federal Gazette, treaties, consultations via SPARQL
- **Official Gazette (SHAB)** — Publication search with rubric filtering via REST
- **opendata.swiss** — Dataset search across Switzerland's open data portal via CKAN
- **Court Decisions (entscheidsuche.ch)** — Full-text search across all 26 cantons via Elasticsearch

**Core features:**
- Dual output mode: human-readable tables in terminal, JSON when piped
- Filesystem caching with source-specific TTLs
- Multilingual support (de, fr, it, en, rm)
- Escape hatches for raw OData and SPARQL queries
- Cross-platform builds (macOS, Linux, Windows)
- Single dependency (Cobra)

### Known Limitations

- Parliament API requires curl fallback due to TLS fingerprint detection
- SHAB publication detail returns raw XML (no structured parsing yet)
- No shell completions bundled yet
- Version defaults to "dev" when built without git tags
