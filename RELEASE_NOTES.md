# Release Notes

## Unreleased

### Added

- `chli swissreg` — Sixth data source: the Swiss Federal Institute of Intellectual Property (IPI) public register at [swissreg.ch](https://www.swissreg.ch), via the unauthenticated JSON search backend behind the official database client. No registration, no API key — the same endpoint the Angular SPA uses.
  - `chli swissreg trademark|patent|design|patent-pub|design-pub <query>` — Search the five record types. Quote the query for exact-phrase matching (e.g. `'"Ovomaltine"'`). Supports `--status aktiv|geloescht`, `--class 30` (comma-separated for multiple Nice/Locarno classes), `--filed-after YYYY-MM-DD`, `--filed-before YYYY-MM-DD`, and `--office CH|DE|AT|...` filters. `--max` up to 64 (backend cap).
  - `chli swissreg search <query> --target=<name>` — Raw search with an explicit target (`chmarke`, `patent`, `design`, `publikationpatent`, `publikationdesign`).
  - `chli swissreg detail <target> <id>` — Look up a single record by its internal URN id (discoverable in the `id` field of search results). Accepts either a bare id (`1206422825`) or a full URN (`urn:ige:schutztitel:chmarke:1206422825`).
  - `chli swissreg image <number-or-hash>` — Renders a trademark with its metadata block plus an ASCII-art version of the image (box-averaged, contrast-stretched; ramp from space to `@`). `--cols N` sets the width, `--out FILE` saves raw bytes, `--url` prints the image URL, `--raw` pipes bytes to stdout (e.g. `| imgcat`). Trademark rows in the search table include a clickable image URL for figurative marks.
  - Pagination beyond `--max 64` is not supported: the backend uses an opaque Transit+JSON cursor that we don't decode. Refine the query instead.
- `chli shab history <number>` — Walks the `lastFosc` back-pointer chain for a Handelsregister publication, printing the timeline of prior FOSC entries for the same legal entity (oldest → newest, with the current entry marked). Each row is a clickable shab.ch link in interactive terminals. `--depth N` caps the number of back-hops; default unlimited.
- `chli shab publication --diff` — Field-level before/after comparison between `commonsActual` and `commonsNew` (name, UID, seat, legal form, address, purpose, auditor). Shows only the fields that changed.
- `chli parl department [--historic]` — List federal departments (EDA, EDI, EJPD, VBS, EFD, WBF, UVEK, BK, …). Falls back to the legacy `ws-old.parlament.ch` endpoint because the current OData service at `ws.parlament.ch` does not expose departments. `--historic` includes end-dated records with `From`/`To` dates.
- `chli parl events [--sessions] [--category <c>] [--limit N] [--all]` — Upcoming parliamentary events from the parlament.ch agenda (sessions, press conferences, ceremonies). Uses the site's anonymous SharePoint Search endpoint, giving structured data (start/end, localized title, category, location) further out than the OData Session entity publishes. Respects `--lang de|fr|it|en` for localized titles and categories.
- `chli parl` (root) — When invoked without a subcommand, prints the current session (or the most recent past + next upcoming session) before the help text. Falls back to the agenda search for the next session when OData hasn't registered it yet.

### Changed

- `chli entscheid search` — Table now includes an `ID` column so results can be passed directly to `entscheid get`/`download` without re-querying the JSON.
- `chli shab search` — Table now includes a `URL` column rendered as an OSC-8 terminal hyperlink when interactive, plain URL when piped.
- `chli shab publication` — Interactive detail for HR publications now extracts structured fields from the XML (company name + UID, seat + legal form, address, auditor, change labels) before the publication text, and prints the canonical shab.ch URL in the header. Previous releases only surfaced the plain text body.
- `api/shab_types.go` — Replaced the spurious `shabContent` wrapper with a direct mapping of `<content>` (which fixes HR publications that previously parsed to empty content) and added types for `commonsNew` / `commonsActual` / `lastFosc` / `transaction.changements`. `registrationOffice` is now a struct instead of a whitespace-blob string.
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
