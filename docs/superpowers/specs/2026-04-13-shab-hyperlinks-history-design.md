# SHAB: hyperlinks, richer XML extraction, revision history

## Goal

Make `chli shab` publications usable as a research surface: clickable links to the
canonical shab.ch page, structured extraction of the commercial-register XML, a
terse diff view for a single publication, and a timeline of prior publications
for the same legal entity via the `lastFosc` back-pointer chain.

## Scope

In scope:

- Add a canonical URL field to every publication (search + detail), rendered as an
  OSC-8 terminal hyperlink when interactive, plain URL when piped.
- Extend the parsed XML model to cover HR-rubric fields that matter: company
  identity, address, purpose, revision company (auditor), `commonsNew` vs
  `commonsActual`, and the `transaction.update.changements` booleans.
- Replace the interactive `shab publication` detail view with a compact,
  field-labelled block for HR rubric. JSON output returns the same structured
  data.
- New `--diff` flag on `shab publication`: prints only fields that differ
  between `commonsActual` and `commonsNew`, one line per change.
- New `shab history <number>` command: follows `lastFosc.lastFoscSequence`
  iteratively back in time and prints a one-line-per-entry timeline (date,
  number, short change summary, URL) oldest → newest, with the current
  publication marked.

Out of scope (YAGNI):

- Structured extraction for non-HR rubrics (SB, KK, AB, …) — those keep the
  current generic text-extraction path plus the new URL.
- Any new external dependency.
- Forward-chain lookup (publications newer than the given one). We only follow
  `lastFosc` backwards because only that pointer is present in the XML.
- Pagination UI for history — the chain is naturally short per entity.

## Canonical URL

Format: `https://shab.ch/#!/search/publications/detail/<uuid>`.

Rendering:

- When `output.IsInteractive()` and `output.ForceJSON` is false: emit OSC-8
  (`\x1b]8;;<url>\x1b\\<label>\x1b]8;;\x1b\\`). Label defaults to publication
  number for the URL column in tables; for the detail-view URL line the label is
  the URL itself.
- Otherwise: print the URL verbatim.

Placement:

- `shab search`: add a `URL` column after `Date`/`Canton`.
- `shab publication`: add a `URL:` line to the header block.
- `shab history`: one trailing `[link]` per row.

Helper: a small `output.Hyperlink(url, label string) string` utility (new) so the
OSC-8 logic is not scattered.

## Richer XML extraction

Extend `api/shab_types.go` with an HR-oriented model. Important correction to
the existing code: the real XML has `publicationText`, `commonsNew`, etc.
directly under `<content>` — there is no `<shabContent>` wrapper. The current
`SHABXMLSHABContent` layer is spurious and the reason HR publications currently
parse to empty content. Drop that layer and make `SHABXMLContent` the direct
mapping for `<content>`:

```go
type SHABXMLContent struct {
    PublicationText SHABXMLText     `xml:"publicationText"`
    Message         string          `xml:"message,omitempty"`
    JournalNumber   string          `xml:"journalNumber,omitempty"`
    JournalDate     string          `xml:"journalDate,omitempty"`
    CommonsNew      *SHABCommons    `xml:"commonsNew,omitempty"`
    CommonsActual   *SHABCommons    `xml:"commonsActual,omitempty"`
    LastFosc        *SHABLastFosc   `xml:"lastFosc,omitempty"`
    Transaction     *SHABTransaction `xml:"transaction,omitempty"`
}

type SHABCommons struct {
    Company  *SHABCompany  `xml:"company,omitempty"`
    Purpose  string        `xml:"purpose,omitempty"`
    Revision *SHABRevision `xml:"revision,omitempty"`
}

type SHABCompany struct {
    Name      string       `xml:"name"`
    UID       string       `xml:"uid,omitempty"`
    Code13    string       `xml:"code13,omitempty"`
    Seat      string       `xml:"seat,omitempty"`
    LegalForm string       `xml:"legalForm,omitempty"`
    Address   *SHABAddress `xml:"address,omitempty"`
}

type SHABAddress struct {
    Street       string `xml:"street,omitempty"`
    HouseNumber  string `xml:"houseNumber,omitempty"`
    SwissZipCode string `xml:"swissZipCode,omitempty"`
    Town         string `xml:"town,omitempty"`
}

type SHABRevision struct {
    OptingOut       bool           `xml:"optingOut"`
    RevisionCompany *SHABRevisionCompany `xml:"revisionCompany,omitempty"`
}

type SHABRevisionCompany struct {
    Name    string `xml:"name"`
    Country string `xml:"country,omitempty"`
    UID     string `xml:"uid,omitempty"`
}

type SHABLastFosc struct {
    Date     string `xml:"lastFoscDate"`
    Number   string `xml:"lastFoscNumber"`
    Sequence string `xml:"lastFoscSequence"` // publication number of the prior entry
}

type SHABTransaction struct {
    Update *SHABTxUpdate `xml:"update,omitempty"`
    // other transaction kinds (creation, deletion) can be added lazily
}

type SHABTxUpdate struct {
    Changements SHABChangements `xml:"changements"`
}

type SHABChangements struct {
    Others             bool `xml:"others"`
    NameChanged        bool `xml:"nameChanged"`
    UIDChanged         bool `xml:"uidChanged"`
    LegalStatusChanged bool `xml:"legalStatusChanged"`
    SeatChanged        bool `xml:"seatChanged"`
    AddressChanged     bool `xml:"addressChanged"`
    PurposeChanged     bool `xml:"purposeChanged"`
    // capitalChanged and statusChanged are nested; model lazily as sub-structs
    // if and when we surface them in the UI.
}
```

Also fix the existing `registrationOffice` bug: currently declared as `string`
in `SHABXMLMeta`, which is why non-HR publications produce a whitespace blob.
Make it a struct matching the XML (`id`, `displayName`, `street`, `town`,
`swissZipCode`, …) — same shape already exists as `SHABOffice` in the search
types; reuse or mirror.

The raw-XML fallback path in `SHABPublicationParsed` stays — if unmarshal fails
we still return the bytes.

## Interactive detail view

For HR rubric, replace the current language-picked plain text with a compact
block (example in the design conversation). Only print sections that have data:

- Header: publication number, date (YYYY-MM-DD), cantons, URL
- Title (language-picked via `output.Lang`, fallback to any available)
- Company line: name + UID
- Seat line: seat + legal-form code
- Address line: street + house number + zip + town
- Auditor line: revision company name + UID (skip when `optingOut` and no
  revision company)
- Changes line: comma-joined human labels derived from `changements` booleans
  (`name`, `seat`, `address`, `purpose`, `legal status`, `others`, …)
- Text: the original `publicationText` body (always keep it — it's the
  authoritative legal summary)

For non-HR rubric, behaviour is unchanged except the URL is added.

JSON output (`--json` or non-interactive): serialize the full parsed struct. No
separate shape for the CLI.

## Diff view (`--diff` flag on `shab publication`)

When set, after the header+URL block, compare `commonsActual` and `commonsNew`
field-by-field and print only differences:

```text
Name:    Old GmbH
      → New AG
Seat:    Zürich
      → Zug
Auditor: (none)
      → Ernst & Young SA
```

If both sides are absent or identical → print `No field-level changes
(see Changes line).` and exit 0.

Implementation: a small `shabDiff(actual, new *SHABCommons) []diffLine`
function in `cmd/shab.go` (or a new `cmd/shab_diff.go` if it grows past ~80
lines). Kept out of `api/` because it's a presentation concern.

## History command (`chli shab history <number>`)

Synopsis:

```text
chli shab history <publicationNumber|uuid> [--depth N]
```

Behaviour:

1. Resolve the starting publication via `SHABResolveID` + `SHABPublicationParsed`.
2. Walk back: while `content.lastFosc.lastFoscSequence` is non-empty and
   `depth < N` (default unlimited), resolve that sequence, parse, append.
3. Reverse the collected slice so oldest is first.
4. Render:

   ```text
   2024-07-11  HR02-1005901234  Neueintragung                       [link]
   2025-03-02  HR02-1006011111  Mutation — Vorstand geändert        [link]
   2026-01-08  HR02-1006532173  Mutation — Sitz geändert            [link]
   2026-04-02  HR02-1006615899  Mutation — andere                   [link]  ← current
   ```

   Change-summary rules:

   - If `transaction.update.changements` is present, emit `Mutation — ` plus a
     comma-joined list of human labels for the true flags.
   - Else if the XML indicates creation/deletion via other transaction children,
     map those to `Neueintragung` / `Löschung`.
   - Else fall back to the publication's title (truncated to 40 chars).

JSON output: array of
`{date, publicationNumber, url, changeSummary, changements, isCurrent}`.

Caching: each hop is an independent `SHABPublication` call, so the existing
1-hour TTL cache applies. No new cache layer needed.

Error handling:

- If a `lastFoscSequence` can't be resolved (deleted, access denied),
  print a `(unresolved: <seq>)` placeholder row and stop walking.
- If the starting publication has no `lastFosc`, print just that one row with a
  note: `No prior FOSC entries referenced by this publication.`

Depth cap: `--depth 0` means unlimited (default). Any positive N caps the hops.

## File layout

- `api/shab_types.go` — extend types as above.
- `api/shab.go` — no new public calls; existing `SHABPublicationParsed` returns
  the richer struct. Optionally add `SHABHistory(id string, depth int)
  ([]*SHABPublicationXML, error)` helper that does the chain walk so the cmd
  layer stays thin.
- `cmd/shab.go` — add URL column, `--diff` flag, wire new render, register
  `shab history`.
- `output/hyperlink.go` (new, small) — OSC-8 helper.
- Tests: extend `api/shab` tests (currently none) with a parsing test over a
  captured HR02 XML fixture checked into `api/testdata/shab_hr02.xml`.

## Testing

- Parse fixture → assert company name/UID, changements flags, lastFosc fields.
- Diff helper: feed two `SHABCommons` with one differing field, assert single
  diff line.
- History command: inject a fake client (existing `api.Client` is already
  constructor-based; parameterise the HTTP layer or write a small round-tripper
  fake). Assert ordering and change-summary mapping.
- Golden-file test for interactive detail render (HR) and for `--diff` output.

## Open questions

None — all design points resolved in brainstorming.
