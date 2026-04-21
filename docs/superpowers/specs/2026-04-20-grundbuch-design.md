# chli grundbuch — design spec

**Date:** 2026-04-20
**Status:** approved, ready for implementation plan
**Scope:** add a new top-level command `chli grundbuch` for Swiss parcel (Liegenschaft / immeuble) and ownership (Grundbuch / registre foncier) lookups, covering all 26 cantons.

---

## 1. Purpose

Swiss cadastre and land-registry data is fragmented across 26 cantonal systems with wildly different access regimes — some cantons publish owner names in a free web viewer, others require a paid Terravis/Intercapi account, a SwissID login, an SMS code to a Swiss mobile phone, or an in-person visit with proof of interest. A CLI user today has no single entry point and no easy way to learn what's possible per canton.

`chli grundbuch` unifies this:

- A **parcel lookup** that works identically across all 26 cantons via the federal aggregated cadastre.
- An **owner lookup** that attempts a live query where a canton offers an unauthenticated public endpoint, and otherwise prints a structured "how to obtain" block (portal URL, auth method, cost, legal basis, turnaround).
- A **capability matrix** (`cantons`, `canton XX`) so users can see the landscape at a glance before spending time on any one canton.

The command is honest about what's possible. It never pretends to return a certified extract (beglaubigter Grundbuchauszug) — those always require a paid, identity-verified order.

## 2. Non-goals

- No scraping of JavaScript-rendered viewers. If owner data is only visible after tile callbacks in a canton's web map, the command reports capability metadata rather than running a headless browser.
- No automated identity flows. The command does not perform SMS verification, SwissID / AGOV / SuisseID login, or Terravis/Intercapi professional authentication. Those flows are *explained*, not executed.
- No certified extracts. Official `Grundbuchauszug` / `extrait du registre foncier` always require the paid, identity-verified canton flow — the CLI only surfaces the path to it.
- No bulk scraping. The command is for looking up one parcel at a time; per-canton rate limits are respected.

## 3. Capability tiering

Every canton is classified into one of five tiers based on what the CLI can actually deliver without authentication.

| Tier | Cantons | Parcel | Owner lookup | How |
|---|---|---|---|---|
| **T1 — Fully public owner** | FR, BS, BL, SZ, TG | ✅ | ✅ live, unauthenticated | Public canton endpoint exposes owner |
| **T2 — Semi-public owner** | UR, GL | ✅ | ✅ limited / rolling out | ÖREB viewer or new Grundstücksinformation |
| **T3 — Free but gated** | ZH, ZG, AG, LU | ✅ | ❌ via CLI — requires SMS to CH mobile or daily quota | Canton portal, free but not automatable |
| **T4 — Free, full eID** | BE | ✅ | ❌ via CLI — requires AGOV (federal eID) | GRUDIS Public |
| **T5 — No public owner** | VD, GE, VS, NE, JU, TI, SH, SG, GR, AR, AI, OW, NW, SO | ✅ | ❌ | Terravis / Intercapi / counter visit |

Parcel/geometry/EGRID lookup is universal — every tier has it via the federal aggregated layer.

**Tier vs. phase.** Tier describes what the canton itself offers. CLI adapter coverage is phased (§7): a T1 canton without an adapter in the current phase falls through to the how-to block, same as a T5 canton. Phase 1 ships with one live adapter (FR).

Tiering notes:
- **VS** has a partial public consultation for digitized communes, but requires a "portail des autorités valaisannes" account — placed in T5 because no zero-auth path exists.
- **GE (SITG)** is famously open for GIS data but deliberately excludes owner from open data — T5.
- Individual canton adapters carry a `Caveat` field so facts flagged "unverified" in research (e.g. exact 2026 tariffs for SH, SG, GR, AR, OW, AI) surface to the user rather than get hard-coded.

## 4. Command surface

```text
chli grundbuch parcel   [--egrid CH…] [--address "…"] [--coord LAT,LON]
                        [--canton XX --number N]
chli grundbuch owner    [--egrid CH…] [--canton XX --number N]
                        [--explain]
chli grundbuch canton   XX
chli grundbuch cantons
```

All respect the global flags `--json`, `--lang`, `--no-cache`, `--refresh`, `--verbose`, `--columns`, `-o format`.

### 4.1 `parcel`

Resolves a parcel by one of four inputs and returns:

- `EGRID` — federal parcel ID
- `Canton` — 2-letter code
- `Municipality` — name + BFS number
- `ParcelNumber` — canton-local parcel number
- `Area` — m²
- `Coord` — WGS84 (lat, lon) and LV95 (E, N)
- `Portal` — URL of the canton's authoritative viewer for this parcel
- `Geometry` — GeoJSON (JSON mode only; suppressed in table)

Resolution order:
1. `--egrid` → `MapServer/find?layer=ch.kantone.cadastralwebmap-farbe&searchField=egris_egrid&searchText=…` on `api3.geo.admin.ch`.
2. `--address` → `SearchServer?origins=parcel,address&searchText=…` then identify by coordinate.
3. `--coord` → `MapServer/identify?layers=all:ch.kantone.cadastralwebmap-farbe&geometry=lon,lat&geometryType=esriGeometryPoint` on `api3.geo.admin.ch`.
4. `--canton XX --number N` → canton-specific AV WFS via `geodienste.ch/services/av` (canton-filtered).

Canton is derived from the parcel's municipality (BFS number → canton), not from the EGRID prefix (EGRID prefix ↔ canton is not a clean mapping).

### 4.2 `owner`

Input: `--egrid` or `--canton XX --number N`. First resolves the parcel (4.1), then routes to the canton's adapter:

- **T1 / T2 with a documented public endpoint that passes probing** → live lookup. Output:

  ```text
  Owner        <name(s)>
  Ownership    <form, e.g. Miteigentum je 1/2>
  Acquired     <date, if exposed>
  EGRID        CH…
  Source       <canton-portal.ch> (unofficial; no legal validity)
  Retrieved    2026-04-20 14:02 UTC
  Certified    → <official order URL>, <portal/auth>, CHF <cost>
  ```
- **T3 / T4 / T5 (or T1/T2 adapter failed)** → structured how-to block:

  ```text
  Canton <XX> does not expose owner data via a public API / endpoint is unavailable.

  To obtain ownership information:
    Portal        <Terravis | Intercapi | canton-specific>
    URL           <canonical URL>
    Auth          <SwissID | Mobile ID | AGOV | SMS to CH mobile | counter visit | professional convention>
    Cost          CHF <amount or range or "unpriced — verify at URL">
    Turnaround    <e.g. same-day counter, 1-week mail>
    Legal basis   ZGB Art. 970 <free text>

  Verified: <YYYY-MM-DD>
  ```

`--explain` always shows the how-to block *in addition to* a live result, so scripts preparing a certified-extract order still get the routing info.

`--json` returns a single object with both `result` (owner data, if any) and `capability` (the how-to metadata).

### 4.3 `canton XX`

Per-canton detail page: tier, parcel endpoint, owner endpoint (if any), authoritative Grundbuchamt URL, auth model, cost, portals (Terravis/Intercapi/RFpublic/GRUDIS/…), legal notes, verification date. `--json` emits the full `CantonCapability` record.

### 4.4 `cantons`

Matrix across all 26 cantons. Table (TTY): code, name, tier, parcel ✓, owner-public status, auth, cost. JSON: array of `CantonCapability` objects.

## 5. Architecture

### 5.1 File layout (follows chli's "one file per source" convention)

```text
cmd/grundbuch.go                Cobra commands: parcel, owner, canton, cantons
api/grundbuch.go                Federated client (geo.admin.ch + geodienste.ch)
api/grundbuch_types.go          Parcel, Owner, CantonCapability, AuthModel, Tier
api/grundbuch_cantons.go        Static capability registry: one record per canton
api/grundbuch_adapters.go       Per-canton live owner fetchers (T1/T2 only,
                                post-probing)
api/grundbuch_test.go           Unit tests (parsing, matrix consistency, mocked HTTP)
```

### 5.2 Federated client (`api/grundbuch.go`)

Uses existing `api/client.go` (`DoRequest`, TLS fingerprint, retry with backoff).

- `SearchAddress(q) -> []Hit`
- `IdentifyParcelByCoord(lat, lon) -> Parcel`
- `FindParcelByEGRID(egrid) -> Parcel`
- `FetchAV(cantonCode) -> AVService` (WMS/WFS base URLs from `geodienste.ch`)
- `CantonForBFS(bfsNum) -> CantonCode` (via a BFS→canton lookup table bundled at build time from a published BFS list)

### 5.3 Capability registry (`api/grundbuch_cantons.go`)

```go
type Tier int
const (
    TierT1 Tier = iota + 1 // fully public owner
    TierT2                 // semi-public / rolling out
    TierT3                 // free but gated (SMS / quota)
    TierT4                 // free eID
    TierT5                 // no public owner
)

type AuthModel string
const (
    AuthNone         AuthModel = "none"
    AuthSMSPhone     AuthModel = "sms-to-ch-mobile"
    AuthAGOV         AuthModel = "agov"
    AuthSwissID      AuthModel = "swissid"
    AuthProfessional AuthModel = "professional-convention"
    AuthCounter      AuthModel = "counter-or-mail"
)

type PortalRef struct {
    Name string
    URL  string
    Type string // "wms" | "wfs" | "rest" | "viewer"
}

type CostSpec struct {
    FixedCHF   *int   // e.g. 20
    MinCHF     *int   // lower bound of range
    MaxCHF     *int   // upper bound of range
    Unpriced   bool   // "verify at URL"
    Notes      string // "+ CHF 1.20 postage", etc.
}

type OwnerEndpoint struct {
    URL      string
    Type     string // "rest" | "wfs-getfeatureinfo" | "wms-getfeatureinfo"
    Verified bool   // did probing confirm it returns owner data server-side?
    Notes    string
}

type CantonCapability struct {
    Code             string                 // "ZH", "BE", ...
    Name             map[string]string      // "de"/"fr"/"it"/"en"
    Tier             Tier
    ParcelPortal     PortalRef
    OwnerPublic      *OwnerEndpoint         // nil for T3..T5
    OwnerOrder       []PortalRef            // Terravis, Intercapi, online form...
    AuthModel        AuthModel
    Cost             CostSpec
    GrundbuchamtURL  string
    LegalNotes       string                 // 1-2 lines re ZGB 970 / Interessennachweis
    Caveats          []string               // "Cost unverified for 2026 — confirm at URL"
    VerifiedAt       string                 // "2026-04-20"
}

var Cantons = map[string]CantonCapability{ /* 26 entries */ }
```

Registry is code, not YAML/JSON, so the compiler enforces completeness when fields are added.

### 5.4 Live adapters (`api/grundbuch_adapters.go`)

Only T1/T2 cantons with a documented endpoint that passes probing. **Phase 1 target: FR (RFpublic)** — the one canton with an explicit, unauthenticated public owner lookup. The others (BS, BL, SZ, TG) are candidates for Phase 2, conditional on a probe confirming the endpoint returns owner data server-side (not only via JS rendering).

```go
type OwnerFetcher func(ctx context.Context, p Parcel) (*Owner, error)

var adapters = map[string]OwnerFetcher{
    "FR": fetchFROwner,
    // BS/BL/SZ/TG added in Phase 2 after probing
}
```

If a canton has no adapter, `owner` falls back to the capability how-to block. If an adapter errors (HTTP 4xx/5xx, parse failure), same fallback — with a `--verbose`-visible error note.

### 5.5 Cache TTLs (`api/cache.go` additions)

| Data | TTL | Rationale |
|---|---|---|
| Parcel geometry / EGRID resolve | 7 days | Cadastre is slow-moving |
| BFS → canton table | build-time | Ships with binary |
| Capability matrix | build-time | Ships with binary |
| Owner lookups | **not cached** | Freshness + legal (stale ownership is worse than none) |

### 5.6 Rate limiting & identification

- `owner` defaults to 1 request/second across all adapters (sleep-between-requests, not token bucket — CLI is single-user by definition).
- User-Agent: `chli/<version> (+https://github.com/salam/chli)` so canton operators can block cleanly if they want.
- If a canton adapter returns 429 or 503, back off and fall through to the how-to block for that invocation.

### 5.7 Errors

- Network / endpoint down → existing retry-with-backoff path, then error to stderr with suggestion.
- Canton adapter returns 403/404 → log at `--verbose`, fall through to the T5-style how-to block (never silently empty).
- EGRID can't be resolved → structured error suggesting `parcel --address` or `--coord`.
- `--canton XX --number N` with unknown canton → list the 26 valid codes.

## 6. Output

- Terminal (TTY): aligned key/value for single records, tables for `cantons`.
- Piped: JSON by default.
- `--json`: forces JSON in terminal.
- All output via `output/` (no direct `fmt.Println`).
- `caveats[]` surfaces unverified fields in both table (footer) and JSON.

## 7. Phasing

**Phase 1 (MVP, one PR):**
1. `parcel` — federated lookup, 4 input modes, works for all 26 cantons.
2. `canton` / `cantons` — static registry populated from research, all 26 cantons.
3. `owner` — prints how-to blocks for every canton. FR gets a live adapter (the one clear public endpoint).

**Phase 2 (follow-up PR, after endpoint probing):** live adapters for BS, BL, SZ, TG if probing confirms server-side owner attribute exposure. If any fail the probe, they stay capability-only.

**Phase 3 (follow-up, low priority):** `--explain` expansions describing SMS / AGOV / Terravis / Intercapi flows; no live implementation of gated flows.

## 8. Testing

- Unit: EGRID parsing, BFS→canton resolution, capability matrix consistency (every canton has required fields or explicit caveats; every tier has a valid auth model).
- Unit: mocked HTTP for FR adapter.
- Integration (manual, behind `TEST_HTTP=1` env var): one live request each to `api3.geo.admin.ch` (parcel), `keycloak.fr.ch/rfpublic/` (FR owner). Skipped in CI by default.

## 9. Legal / ethical framing

- ZGB Art. 970 makes owner name federally public *in principle*, but cantons implement differently and most require a formal Interessennachweis or paid extract.
- The CLI never claims to deliver a certified extract. Every live owner result carries `Source: <canton-portal> (unofficial; no legal validity)` in the output.
- The CLI respects canton-level opt-outs — where a canton indicates owner data is suppressed for a specific parcel (e.g. BS owner blocking), the adapter reports "owner suppressed on request of owner" rather than inventing data.
- Owner data is not cached (§5.5).
- Rate-limited and identified User-Agent (§5.6), so canton operators have a clean path to block if they want.

## 10. Open items flagged during research (to re-verify at implementation time)

- Exact character count of EGRID (10 vs 12 vs 14 digits after `CH`) — consult the federal Leitfaden PDF before parsing validation.
- Whether `ch.kantone.cadastralwebmap-farbe` coverage is 100% canton coverage today.
- Exact 2026 CHF tariffs for SH, SG, GR, AR, OW, AI, LU, SO, BS (some published only in cantonal regulations).
- Exact WMS/WFS GetCapabilities hostname for SG (`ktsg`) and the GR / TG geocatalog endpoint strings.
- Current state of owner-name exposure in the UR ÖREB viewer (post-dnip.ch 2021 disclosure — may have been further restricted).
- Whether BS / BL / SZ / TG GetFeatureInfo on their cadastre WMS layers actually returns owner attribute server-side or only renders it in the JS viewer. **This determines Phase 2 scope.**

Each open item either blocks a specific piece of work (parsing, adapter) or surfaces as a `Caveat` to the user in the capability output — they do not block Phase 1 shipping.
