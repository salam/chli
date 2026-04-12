# TODO

## High Priority

- [x] Add shell completion generation (`chli completion bash/zsh/fish/powershell`)
- [x] Parse SHAB publication XML into structured output instead of raw XML dump
- [x] Add `--version` output to include build date and commit hash
- [x] Write unit tests for API clients and output formatting
- [x] Add CI pipeline (GitHub Actions) for automated builds and releases

## Features

- [x] `chli fedlex diff` — Compare two versions of the same SR law text
- [x] `chli parl vote <id>` — Detailed vote results with per-member breakdown, and per-party breakdown
- [x] `chli parl vote <id>` — Offer visualization of the parliament seats in the room with yes/no/etc. votes
- [x] `chli parl session` — List and browse sessions with agenda
- [x] `chli entscheid download <id>` — Direct PDF download to disk
- [x] `chli opendata download <dataset> <resource>` — Download dataset resources
- [x] Configurable output columns per command
- [x] Bookmark/watch functionality for parliament business items
- [x] RSS/feed mode for new SHAB publications or court decisions
- [x] `chli parl department` — List federal departments via legacy ws-old.parlament.ch fallback (OData doesn't expose them)
- [x] `chli parl events` — List upcoming agenda events (sessions, press conferences, ceremonies) via the parlament.ch SharePoint Search endpoint
- [x] Surface future-scheduled sessions (that OData has not yet registered) in `chli parl` and `chli parl session` by merging the agenda search results

## Quality of Life

- [x] `--output csv` and `--output tsv` for spreadsheet workflows
- [x] `--quiet` flag to suppress headers and only output data rows
- [x] Colorized table output with semantic highlighting (status, dates, parties)
- [x] Interactive pager for long result sets (less-style)
- [x] Retry with backoff on transient HTTP errors
- [x] Cache management commands (`chli cache clear`, `chli cache stats`)

## Technical

- [x] Replace curl fallback for Parliament API with custom TLS configuration
- [x] Add request timeout configuration (currently uses http.Client defaults)
- [x] Structured logging with `--verbose` / `--debug` flags
- [x] Integration tests against live APIs (with cache fixtures)
- [x] Homebrew formula for macOS installation
- [x] Nix package / flake
- [x] goreleaser configuration for automated GitHub Releases

## Follow-ups

- [ ] Watch for a `Department` entity in `ws.parlament.ch` OData `$metadata`; migrate `chli parl department` off the legacy endpoint when it appears.
- [ ] Watch for future sessions to appear in the OData `Session` entity earlier; trim the agenda-search merge in `fetchSessionsAround` once the lag is gone.
- [ ] Consider caching the agenda SharePoint `ProcessQuery` response (currently uncached because the form digest is short-lived).
