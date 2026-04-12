# SHAB hyperlinks, richer XML, and history — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `chli shab` a usable research surface: clickable shab.ch links, properly parsed HR XML (company / address / purpose / auditor / transaction), a `--diff` view of a single publication, and a `shab history` timeline that walks the `lastFosc` back-pointer chain.

**Architecture:** Fix the XML model in `api/shab_types.go` (drop the bogus `shabContent` wrapper, add HR-oriented fields). Add an OSC-8 hyperlink helper in `output/`. Render logic stays in `cmd/shab.go`, with `diff` / `history` helpers extracted to `cmd/shab_diff.go` and `cmd/shab_history.go` when they grow past trivial. One new public API helper: `Client.SHABHistory`.

**Tech Stack:** Go 1.22+, `encoding/xml`, cobra, existing `api.Client` HTTP+cache layer.

Spec: [docs/superpowers/specs/2026-04-13-shab-hyperlinks-history-design.md](../specs/2026-04-13-shab-hyperlinks-history-design.md)

---

## File layout

| Path | Role |
|------|------|
| `output/hyperlink.go` | **Create.** OSC-8 terminal hyperlink helper. |
| `output/hyperlink_test.go` | **Create.** Tests for OSC-8 emission / fallback. |
| `api/shab_types.go` | **Modify.** Drop `SHABXMLSHABContent` wrapper; add HR model (`SHABCommons`, `SHABCompany`, `SHABAddress`, `SHABRevision`, `SHABRevisionCompany`, `SHABLastFosc`, `SHABTransaction`, `SHABTxUpdate`, `SHABChangements`); fix `registrationOffice` type in meta. |
| `api/testdata/shab_hr02.xml` | **Create.** Captured real HR02 XML fixture for parser tests. |
| `api/shab_test.go` | **Create.** Parse fixture, assert key fields. |
| `api/shab.go` | **Modify.** Add `SHABHistory(id string, depth int)` helper that walks `lastFosc`. |
| `cmd/shab.go` | **Modify.** URL column in search; richer HR detail block; URL line; `--diff` flag; register `history` subcommand. |
| `cmd/shab_diff.go` | **Create.** `shabDiffCommons` helper + tests. |
| `cmd/shab_diff_test.go` | **Create.** |
| `cmd/shab_history.go` | **Create.** History subcommand + change-summary helper. |
| `cmd/shab_history_test.go` | **Create.** Tests for change-summary helper. |

---

## Task 1: OSC-8 hyperlink helper

**Files:**
- Create: `output/hyperlink.go`
- Create: `output/hyperlink_test.go`

- [ ] **Step 1: Write the failing test**

```go
package output

import "testing"

func TestHyperlinkInteractive(t *testing.T) {
	prev := ForceJSON
	ForceJSON = false
	t.Cleanup(func() { ForceJSON = prev })

	got := hyperlinkFor(true, "https://example.com", "label")
	want := "\x1b]8;;https://example.com\x1b\\label\x1b]8;;\x1b\\"
	if got != want {
		t.Fatalf("interactive: got %q want %q", got, want)
	}
}

func TestHyperlinkNonInteractive(t *testing.T) {
	if got := hyperlinkFor(false, "https://example.com", "label"); got != "https://example.com" {
		t.Fatalf("non-interactive: got %q, want bare URL", got)
	}
}

func TestHyperlinkEmptyLabelUsesURL(t *testing.T) {
	got := hyperlinkFor(true, "https://example.com", "")
	if !contains(got, "https://example.com") {
		t.Fatalf("expected URL in body, got %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./output/ -run TestHyperlink -v`
Expected: FAIL — `hyperlinkFor` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package output

// Hyperlink returns an OSC-8 terminal hyperlink when stdout is interactive,
// or the plain URL otherwise. If label is empty, the URL is used as the label.
func Hyperlink(url, label string) string {
	return hyperlinkFor(IsInteractive(), url, label)
}

func hyperlinkFor(interactive bool, url, label string) string {
	if url == "" {
		return ""
	}
	if label == "" {
		label = url
	}
	if !interactive {
		return url
	}
	return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./output/ -run TestHyperlink -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add output/hyperlink.go output/hyperlink_test.go
git commit -m "Add OSC-8 terminal hyperlink helper"
```

---

## Task 2: Capture HR02 XML fixture

**Files:**
- Create: `api/testdata/shab_hr02.xml`

- [ ] **Step 1: Pull real XML**

Run:
```bash
mkdir -p api/testdata
go run ./cmd/dumpxml 2>/dev/null || true  # no-op if absent
# Use the existing binary — it already caches on disk.
# Pick any HR02 publication from a live search:
./chli shab search "migros" --size 10 | \
  python3 -c 'import json,sys
d=json.load(sys.stdin)
for p in d["content"]:
    if p["meta"]["rubric"]=="HR":
        print(p["meta"]["id"]);break' > /tmp/shab_id
ID=$(cat /tmp/shab_id)
cat > /tmp/shab_dump.go <<'EOF'
package main
import (
	"fmt"; "os"
	"github.com/matthiasak/chli/api"
)
func main() {
	c,_ := api.NewClient()
	data, err := c.SHABPublication(os.Args[1])
	if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	os.Stdout.Write(data)
}
EOF
go run /tmp/shab_dump.go "$ID" > api/testdata/shab_hr02.xml
rm /tmp/shab_dump.go /tmp/shab_id
```

Expected: `api/testdata/shab_hr02.xml` exists and starts with `<?xml`.

- [ ] **Step 2: Sanity check**

Run: `head -1 api/testdata/shab_hr02.xml && wc -l api/testdata/shab_hr02.xml`
Expected: `<?xml version='1.0' ...>` on line 1, file > 50 lines.

- [ ] **Step 3: Commit**

```bash
git add api/testdata/shab_hr02.xml
git commit -m "Add HR02 XML fixture for SHAB parser tests"
```

---

## Task 3: Fix XML model — drop shabContent wrapper, add HR types

**Files:**
- Modify: `api/shab_types.go`
- Create: `api/shab_test.go`

- [ ] **Step 1: Write the failing test**

`api/shab_test.go`:

```go
package api

import (
	"encoding/xml"
	"os"
	"testing"
)

func TestSHABParseHR02Fixture(t *testing.T) {
	data, err := os.ReadFile("testdata/shab_hr02.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var pub SHABPublicationXML
	if err := xml.Unmarshal(data, &pub); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if pub.Meta.PublicationNumber == "" {
		t.Errorf("missing publicationNumber")
	}
	if pub.Meta.Rubric != "HR" {
		t.Errorf("rubric = %q, want HR", pub.Meta.Rubric)
	}
	if pub.Meta.RegistrationOffice == nil || pub.Meta.RegistrationOffice.DisplayName == "" {
		t.Errorf("registrationOffice not parsed as struct: %+v", pub.Meta.RegistrationOffice)
	}
	if pub.Content.PublicationText.Body == "" {
		t.Errorf("publicationText body empty")
	}
	if pub.Content.CommonsNew == nil || pub.Content.CommonsNew.Company == nil {
		t.Fatalf("commonsNew.company missing")
	}
	if pub.Content.CommonsNew.Company.Name == "" {
		t.Errorf("company name empty")
	}
	if pub.Content.LastFosc == nil {
		t.Errorf("lastFosc missing")
	}
}
```

Note: the test asserts `PublicationText.Body` rather than the old multilingual shape — HR02 XML puts the text directly in the `<publicationText>` element as CDATA, not wrapped in `<de>`/`<fr>`. We'll handle both shapes.

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./api/ -run TestSHABParseHR02Fixture -v`
Expected: FAIL (compile error — types don't exist).

- [ ] **Step 3: Rewrite `api/shab_types.go`**

Replace the whole file with:

```go
package api

import (
	"encoding/xml"

	"github.com/matthiasak/chli/output"
)

// SHABPublicationXML is the parsed XML of a single SHAB publication.
type SHABPublicationXML struct {
	XMLName xml.Name       `xml:"publication" json:"-"`
	Meta    SHABXMLMeta    `xml:"meta" json:"meta"`
	Content SHABXMLContent `xml:"content" json:"content"`
}

// SHABXMLMeta holds metadata fields from the publication XML.
type SHABXMLMeta struct {
	ID                 string              `xml:"id" json:"id,omitempty"`
	PublicationNumber  string              `xml:"publicationNumber" json:"publicationNumber"`
	PublicationState   string              `xml:"publicationState" json:"publicationState,omitempty"`
	PublicationDate    string              `xml:"publicationDate" json:"publicationDate"`
	Rubric             string              `xml:"rubric" json:"rubric"`
	SubRubric          string              `xml:"subRubric" json:"subRubric,omitempty"`
	Language           string              `xml:"language" json:"language,omitempty"`
	Cantons            string              `xml:"cantons" json:"cantons,omitempty"`
	LegalRemedy        string              `xml:"legalRemedy" json:"legalRemedy,omitempty"`
	Title              *SHABXMLTitle       `xml:"title" json:"title,omitempty"`
	RegistrationOffice *SHABXMLOffice      `xml:"registrationOffice" json:"registrationOffice,omitempty"`
}

// SHABXMLTitle holds the multilingual <title> block.
type SHABXMLTitle struct {
	DE string `xml:"de" json:"de,omitempty"`
	FR string `xml:"fr" json:"fr,omitempty"`
	IT string `xml:"it" json:"it,omitempty"`
	EN string `xml:"en" json:"en,omitempty"`
}

// Pick returns the title in the preferred language, falling back to any available.
func (t *SHABXMLTitle) Pick(lang string) string {
	if t == nil {
		return ""
	}
	switch lang {
	case "fr":
		if t.FR != "" {
			return t.FR
		}
	case "it":
		if t.IT != "" {
			return t.IT
		}
	case "en":
		if t.EN != "" {
			return t.EN
		}
	}
	if t.DE != "" {
		return t.DE
	}
	if t.FR != "" {
		return t.FR
	}
	if t.IT != "" {
		return t.IT
	}
	return t.EN
}

// SHABXMLOffice is the structured <registrationOffice> block.
type SHABXMLOffice struct {
	ID           string `xml:"id" json:"id,omitempty"`
	DisplayName  string `xml:"displayName" json:"displayName,omitempty"`
	Street       string `xml:"street" json:"street,omitempty"`
	StreetNumber string `xml:"streetNumber" json:"streetNumber,omitempty"`
	SwissZipCode string `xml:"swissZipCode" json:"swissZipCode,omitempty"`
	Town         string `xml:"town" json:"town,omitempty"`
}

// SHABXMLContent is the <content> body. For HR publications it contains the
// company commons, transaction, and lastFosc pointer directly (no shabContent
// wrapper despite what older schemas suggested).
type SHABXMLContent struct {
	PublicationText SHABXMLText      `xml:"publicationText" json:"publicationText,omitempty"`
	Message         string           `xml:"message" json:"message,omitempty"`
	JournalNumber   string           `xml:"journalNumber" json:"journalNumber,omitempty"`
	JournalDate     string           `xml:"journalDate" json:"journalDate,omitempty"`
	CommonsNew      *SHABCommons     `xml:"commonsNew" json:"commonsNew,omitempty"`
	CommonsActual   *SHABCommons     `xml:"commonsActual" json:"commonsActual,omitempty"`
	LastFosc        *SHABLastFosc    `xml:"lastFosc" json:"lastFosc,omitempty"`
	Transaction     *SHABTransaction `xml:"transaction" json:"transaction,omitempty"`
}

// SHABXMLText carries either multilingual children (<de>/<fr>/<it>) or a plain
// text body (HR publications put the text directly under <publicationText>).
type SHABXMLText struct {
	Body string `xml:",chardata" json:"body,omitempty"`
	DE   string `xml:"de" json:"de,omitempty"`
	FR   string `xml:"fr" json:"fr,omitempty"`
	IT   string `xml:"it" json:"it,omitempty"`
}

// PickText returns the text body in the preferred language, preferring the
// language children when present and falling back to the plain body.
func (t SHABXMLText) PickText(lang string) string {
	switch lang {
	case "fr":
		if t.FR != "" {
			return t.FR
		}
	case "it":
		if t.IT != "" {
			return t.IT
		}
	}
	if t.DE != "" {
		return t.DE
	}
	if t.FR != "" {
		return t.FR
	}
	if t.IT != "" {
		return t.IT
	}
	return t.Body
}

// SHABCommons is the shared structure of commonsNew / commonsActual.
type SHABCommons struct {
	Company  *SHABCompany  `xml:"company" json:"company,omitempty"`
	Purpose  string        `xml:"purpose" json:"purpose,omitempty"`
	Revision *SHABRevision `xml:"revision" json:"revision,omitempty"`
}

// SHABCompany represents the company block inside commons*.
type SHABCompany struct {
	Name      string       `xml:"name" json:"name"`
	UID       string       `xml:"uid" json:"uid,omitempty"`
	Code13    string       `xml:"code13" json:"code13,omitempty"`
	Seat      string       `xml:"seat" json:"seat,omitempty"`
	LegalForm string       `xml:"legalForm" json:"legalForm,omitempty"`
	Address   *SHABAddress `xml:"address" json:"address,omitempty"`
}

// SHABAddress is the street address inside a company block.
type SHABAddress struct {
	Street       string `xml:"street" json:"street,omitempty"`
	HouseNumber  string `xml:"houseNumber" json:"houseNumber,omitempty"`
	SwissZipCode string `xml:"swissZipCode" json:"swissZipCode,omitempty"`
	Town         string `xml:"town" json:"town,omitempty"`
}

// SHABRevision is the revision (audit) block inside a commons.
type SHABRevision struct {
	OptingOut       bool                 `xml:"optingOut" json:"optingOut"`
	RevisionCompany *SHABRevisionCompany `xml:"revisionCompany" json:"revisionCompany,omitempty"`
}

// SHABRevisionCompany is the auditor company.
type SHABRevisionCompany struct {
	Name    string `xml:"name" json:"name"`
	Country string `xml:"country" json:"country,omitempty"`
	UID     string `xml:"uid" json:"uid,omitempty"`
}

// SHABLastFosc is the back-pointer to the previous FOSC publication.
type SHABLastFosc struct {
	Date     string `xml:"lastFoscDate" json:"date,omitempty"`
	Number   string `xml:"lastFoscNumber" json:"number,omitempty"`
	Sequence string `xml:"lastFoscSequence" json:"sequence,omitempty"`
}

// SHABTransaction describes the transaction kind recorded in this publication.
type SHABTransaction struct {
	Update   *SHABTxUpdate `xml:"update" json:"update,omitempty"`
	Creation *struct{}     `xml:"creation" json:"creation,omitempty"`
	Deletion *struct{}     `xml:"deletion" json:"deletion,omitempty"`
}

// SHABTxUpdate is the transaction body for mutations.
type SHABTxUpdate struct {
	Changements SHABChangements `xml:"changements" json:"changements"`
}

// SHABChangements lists the top-level changed flags in an update transaction.
type SHABChangements struct {
	Others             bool `xml:"others" json:"others"`
	NameChanged        bool `xml:"nameChanged" json:"nameChanged"`
	UIDChanged         bool `xml:"uidChanged" json:"uidChanged"`
	LegalStatusChanged bool `xml:"legalStatusChanged" json:"legalStatusChanged"`
	SeatChanged        bool `xml:"seatChanged" json:"seatChanged"`
	AddressChanged     bool `xml:"addressChanged" json:"addressChanged"`
	PurposeChanged     bool `xml:"purposeChanged" json:"purposeChanged"`
}

// ChangedLabels returns human labels (language-neutral English for now) for the
// flags that are true, in a stable order. Empty slice when no flags are set.
func (c SHABChangements) ChangedLabels() []string {
	var out []string
	if c.NameChanged {
		out = append(out, "name")
	}
	if c.UIDChanged {
		out = append(out, "UID")
	}
	if c.LegalStatusChanged {
		out = append(out, "legal status")
	}
	if c.SeatChanged {
		out = append(out, "seat")
	}
	if c.AddressChanged {
		out = append(out, "address")
	}
	if c.PurposeChanged {
		out = append(out, "purpose")
	}
	if c.Others {
		out = append(out, "others")
	}
	return out
}

// SHABSearchResult is the top-level response from the SHAB publications search.
type SHABSearchResult struct {
	Content     []SHABPublication `json:"content"`
	Total       int               `json:"total"`
	PageRequest SHABPageRequest   `json:"pageRequest"`
}

// SHABPublication represents a single publication entry from search.
type SHABPublication struct {
	Meta SHABMeta `json:"meta"`
}

// SHABMeta holds metadata for a SHAB publication from search results.
type SHABMeta struct {
	ID                 string                  `json:"id"`
	Rubric             string                  `json:"rubric"`
	SubRubric          string                  `json:"subRubric"`
	Language           string                  `json:"language"`
	PublicationNumber  string                  `json:"publicationNumber"`
	PublicationState   string                  `json:"publicationState"`
	PublicationDate    string                  `json:"publicationDate"`
	ExpirationDate     string                  `json:"expirationDate"`
	Title              output.MultilingualText `json:"title"`
	Cantons            []string                `json:"cantons"`
	RegistrationOffice *SHABOffice             `json:"registrationOffice,omitempty"`
}

// SHABOffice represents a registration office in search results.
type SHABOffice struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	Street       string `json:"street,omitempty"`
	Town         string `json:"town,omitempty"`
	SwissZipCode string `json:"swissZipCode,omitempty"`
}

// SHABPageRequest holds pagination info from the response.
type SHABPageRequest struct {
	Page int `json:"page"`
	Size int `json:"size"`
}
```

- [ ] **Step 4: Update `cmd/shab.go` call-sites that reference the old shape**

The existing `shab publication` code accesses `pub.Content.SHABContent.PublicationText` and `pub.Content.SHABContent.Message`. Replace those with `pub.Content.PublicationText` / `pub.Content.Message`:

```go
txt := pub.Content.PublicationText
text := txt.PickText(output.Lang)
if text == "" && pub.Content.Message != "" {
    text = pub.Content.Message
}
```

(This is a temporary bridge — Task 5 rewrites the whole render.)

- [ ] **Step 5: Build and run the parse test**

Run: `go build ./... && go test ./api/ -run TestSHABParseHR02Fixture -v`
Expected: PASS.

- [ ] **Step 6: Run all tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/shab_types.go api/shab_test.go cmd/shab.go
git commit -m "Rewrite SHAB XML model to cover HR fields and fix office parsing"
```

---

## Task 4: Canonical URL helper

**Files:**
- Modify: `api/shab.go` (add `SHABPublicationURL`)
- Create: `api/shab_url_test.go`

- [ ] **Step 1: Write the failing test**

`api/shab_url_test.go`:

```go
package api

import "testing"

func TestSHABPublicationURL(t *testing.T) {
	got := SHABPublicationURL("c27f4008-2dbc-4da4-af9e-9be31ea16ec2")
	want := "https://shab.ch/#!/search/publications/detail/c27f4008-2dbc-4da4-af9e-9be31ea16ec2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if SHABPublicationURL("") != "" {
		t.Fatalf("empty id should yield empty url")
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./api/ -run TestSHABPublicationURL -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Add the helper to `api/shab.go`**

```go
// SHABPublicationURL returns the public shab.ch detail URL for a publication UUID.
func SHABPublicationURL(uuid string) string {
	if uuid == "" {
		return ""
	}
	return "https://shab.ch/#!/search/publications/detail/" + uuid
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run TestSHABPublicationURL -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/shab.go api/shab_url_test.go
git commit -m "Add SHABPublicationURL helper"
```

---

## Task 5: Hyperlinked search + richer publication render

**Files:**
- Modify: `cmd/shab.go`

- [ ] **Step 1: Add a URL column to search output**

Locate the `shabSearchCmd.RunE` function. Change the `headers` slice and the row builder to include the URL:

```go
headers := []string{"Number", "Rubric", "Title", "Date", "Canton", "URL"}
rows := make([][]string, 0, len(result.Content))
for _, pub := range result.Content {
    m := pub.Meta
    title := output.Truncate(m.Title.Pick(output.Lang), 60)
    date := ""
    if len(m.PublicationDate) >= 10 {
        date = m.PublicationDate[:10]
    }
    cantons := strings.Join(m.Cantons, ",")
    url := api.SHABPublicationURL(m.ID)
    rows = append(rows, []string{
        m.PublicationNumber,
        m.Rubric,
        title,
        date,
        cantons,
        output.Hyperlink(url, m.PublicationNumber),
    })
}
```

- [ ] **Step 2: Rewrite the interactive detail render in `shabPublicationCmd.RunE`**

Replace the block starting with `// Interactive display` and ending before `return nil` with:

```go
// Interactive display
m := pub.Meta
url := api.SHABPublicationURL(m.ID)

fmt.Printf("Publication:  %s\n", m.PublicationNumber)
if m.PublicationDate != "" {
    fmt.Printf("Date:         %s\n", m.PublicationDate)
}
if m.Rubric != "" {
    label := m.Rubric
    if m.SubRubric != "" {
        label += " / " + m.SubRubric
    }
    fmt.Printf("Rubric:       %s\n", label)
}
if m.Cantons != "" {
    fmt.Printf("Cantons:      %s\n", m.Cantons)
}
if title := m.Title.Pick(output.Lang); title != "" {
    fmt.Printf("Title:        %s\n", title)
}
if m.RegistrationOffice != nil && m.RegistrationOffice.DisplayName != "" {
    fmt.Printf("Office:       %s\n", m.RegistrationOffice.DisplayName)
}
if url != "" {
    fmt.Printf("URL:          %s\n", output.Hyperlink(url, url))
}
fmt.Println()

// HR-specific structured block
if cn := pub.Content.CommonsNew; cn != nil && cn.Company != nil {
    co := cn.Company
    fmt.Printf("Company:      %s", co.Name)
    if co.UID != "" {
        fmt.Printf("  (%s)", co.UID)
    }
    fmt.Println()
    if co.Seat != "" || co.LegalForm != "" {
        line := co.Seat
        if co.LegalForm != "" {
            if line != "" {
                line += "  "
            }
            line += "legal form " + co.LegalForm
        }
        fmt.Printf("Seat:         %s\n", line)
    }
    if a := co.Address; a != nil {
        parts := []string{}
        if a.Street != "" {
            s := a.Street
            if a.HouseNumber != "" {
                s += " " + a.HouseNumber
            }
            parts = append(parts, s)
        }
        if a.SwissZipCode != "" || a.Town != "" {
            parts = append(parts, strings.TrimSpace(a.SwissZipCode+" "+a.Town))
        }
        if len(parts) > 0 {
            fmt.Printf("Address:      %s\n", strings.Join(parts, ", "))
        }
    }
    if cn.Revision != nil && cn.Revision.RevisionCompany != nil {
        rc := cn.Revision.RevisionCompany
        line := rc.Name
        if rc.UID != "" {
            line += "  (" + rc.UID + ")"
        }
        fmt.Printf("Auditor:      %s\n", line)
    }
    fmt.Println()
}

if tx := pub.Content.Transaction; tx != nil && tx.Update != nil {
    if labels := tx.Update.Changements.ChangedLabels(); len(labels) > 0 {
        fmt.Printf("Changes:      %s\n\n", strings.Join(labels, ", "))
    }
}

// Publication text (authoritative legal summary)
text := pub.Content.PublicationText.PickText(output.Lang)
if text == "" && pub.Content.Message != "" {
    text = pub.Content.Message
}
if text != "" {
    fmt.Println(strings.TrimSpace(text))
} else {
    fmt.Println(string(raw))
}
return nil
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Smoke test**

Run:
```bash
./chli shab search "migros" --size 3
./chli shab publication HR02-1006615899
```

Expected:
- Search table has a `URL` column; in a terminal the last column is a clickable link.
- Publication detail shows `Company`, `Seat`, `Address`, `Auditor`, `Changes`, then the publication text.
- JSON mode (`| cat`) shows the full extended struct including `commonsNew`, `commonsActual`, `lastFosc`, `transaction`.

- [ ] **Step 5: Commit**

```bash
git add cmd/shab.go
git commit -m "Show publication URL and HR-rich detail block in shab output"
```

---

## Task 6: `--diff` flag

**Files:**
- Create: `cmd/shab_diff.go`
- Create: `cmd/shab_diff_test.go`
- Modify: `cmd/shab.go` (wire the flag)

- [ ] **Step 1: Write the failing test**

`cmd/shab_diff_test.go`:

```go
package cmd

import (
	"strings"
	"testing"

	"github.com/matthiasak/chli/api"
)

func TestDiffCommonsDetectsSeatChange(t *testing.T) {
	old := &api.SHABCommons{Company: &api.SHABCompany{Name: "Acme AG", Seat: "Zürich"}}
	new_ := &api.SHABCommons{Company: &api.SHABCompany{Name: "Acme AG", Seat: "Zug"}}
	lines := diffCommons(old, new_)
	if len(lines) != 1 {
		t.Fatalf("got %d diff lines, want 1: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0].Field, "Seat") ||
		lines[0].Old != "Zürich" || lines[0].New != "Zug" {
		t.Errorf("unexpected diff line: %+v", lines[0])
	}
}

func TestDiffCommonsIdenticalIsEmpty(t *testing.T) {
	c := &api.SHABCommons{Company: &api.SHABCompany{Name: "Acme", Seat: "Zug"}}
	if lines := diffCommons(c, c); len(lines) != 0 {
		t.Fatalf("expected no diffs, got %v", lines)
	}
}

func TestDiffCommonsNilSide(t *testing.T) {
	if lines := diffCommons(nil, nil); len(lines) != 0 {
		t.Fatalf("nil/nil should be empty, got %v", lines)
	}
}

func TestDiffCommonsAuditor(t *testing.T) {
	old := &api.SHABCommons{}
	new_ := &api.SHABCommons{
		Revision: &api.SHABRevision{
			RevisionCompany: &api.SHABRevisionCompany{Name: "Ernst & Young SA"},
		},
	}
	lines := diffCommons(old, new_)
	if len(lines) != 1 || lines[0].Field != "Auditor" ||
		lines[0].Old != "(none)" || lines[0].New != "Ernst & Young SA" {
		t.Errorf("unexpected: %+v", lines)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./cmd/ -run TestDiffCommons -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `cmd/shab_diff.go`**

```go
package cmd

import (
	"fmt"
	"strings"

	"github.com/matthiasak/chli/api"
)

// diffLine is a single field-level difference between two SHABCommons.
type diffLine struct {
	Field string
	Old   string
	New   string
}

// diffCommons returns a list of human-readable diff lines between two commons
// blocks. Fields absent on one side are rendered as "(none)".
func diffCommons(a, b *api.SHABCommons) []diffLine {
	var out []diffLine
	aCo, bCo := companyOf(a), companyOf(b)
	push := func(field, old, new string) {
		if old == new {
			return
		}
		if old == "" {
			old = "(none)"
		}
		if new == "" {
			new = "(none)"
		}
		out = append(out, diffLine{Field: field, Old: old, New: new})
	}

	push("Name", aCo.Name, bCo.Name)
	push("UID", aCo.UID, bCo.UID)
	push("Seat", aCo.Seat, bCo.Seat)
	push("Legal form", aCo.LegalForm, bCo.LegalForm)
	push("Address", addressOneLine(aCo.Address), addressOneLine(bCo.Address))
	push("Purpose", purposeOf(a), purposeOf(b))
	push("Auditor", auditorOf(a), auditorOf(b))
	return out
}

func companyOf(c *api.SHABCommons) api.SHABCompany {
	if c == nil || c.Company == nil {
		return api.SHABCompany{}
	}
	return *c.Company
}

func purposeOf(c *api.SHABCommons) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Purpose)
}

func auditorOf(c *api.SHABCommons) string {
	if c == nil || c.Revision == nil || c.Revision.RevisionCompany == nil {
		return ""
	}
	return c.Revision.RevisionCompany.Name
}

func addressOneLine(a *api.SHABAddress) string {
	if a == nil {
		return ""
	}
	parts := []string{}
	if a.Street != "" {
		s := a.Street
		if a.HouseNumber != "" {
			s += " " + a.HouseNumber
		}
		parts = append(parts, s)
	}
	if a.SwissZipCode != "" || a.Town != "" {
		parts = append(parts, strings.TrimSpace(a.SwissZipCode+" "+a.Town))
	}
	return strings.Join(parts, ", ")
}

// printDiff writes the diff lines to stdout in before/after format.
func printDiff(lines []diffLine) {
	if len(lines) == 0 {
		fmt.Println("No field-level changes (see Changes line).")
		return
	}
	for _, d := range lines {
		fmt.Printf("%-10s %s\n", d.Field+":", d.Old)
		fmt.Printf("%-10s → %s\n", "", d.New)
	}
}
```

- [ ] **Step 4: Wire the `--diff` flag in `cmd/shab.go`**

In `shabPublicationCmd.RunE`, right after the interactive header/HR block and before the publication text, insert:

```go
if diff, _ := cmd.Flags().GetBool("diff"); diff {
    printDiff(diffCommons(pub.Content.CommonsActual, pub.Content.CommonsNew))
    fmt.Println()
}
```

In the `init()` function, add:

```go
shabPublicationCmd.Flags().Bool("diff", false, "Show field-level diff between previous and current state (HR only)")
```

- [ ] **Step 5: Run tests**

Run: `go test ./cmd/ -run TestDiffCommons -v`
Expected: PASS (all four).

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Smoke test**

Run: `./chli shab publication HR02-1006615899 --diff`
Expected: either a short list of `Field: old → new` lines, or `No field-level changes (see Changes line).`.

- [ ] **Step 7: Commit**

```bash
git add cmd/shab.go cmd/shab_diff.go cmd/shab_diff_test.go
git commit -m "Add --diff flag for SHAB publication commons comparison"
```

---

## Task 7: `shab history` command

**Files:**
- Modify: `api/shab.go` (add `SHABHistory` helper)
- Create: `cmd/shab_history.go`
- Create: `cmd/shab_history_test.go`
- Modify: `cmd/shab.go` (register subcommand)

- [ ] **Step 1: Write the failing test for the change-summary helper**

`cmd/shab_history_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/matthiasak/chli/api"
)

func TestChangeSummaryCreation(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Content: api.SHABXMLContent{
			Transaction: &api.SHABTransaction{Creation: &struct{}{}},
		},
	}
	if got := changeSummary(pub); got != "Neueintragung" {
		t.Errorf("got %q, want Neueintragung", got)
	}
}

func TestChangeSummaryDeletion(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Content: api.SHABXMLContent{
			Transaction: &api.SHABTransaction{Deletion: &struct{}{}},
		},
	}
	if got := changeSummary(pub); got != "Löschung" {
		t.Errorf("got %q, want Löschung", got)
	}
}

func TestChangeSummaryUpdate(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Content: api.SHABXMLContent{
			Transaction: &api.SHABTransaction{
				Update: &api.SHABTxUpdate{
					Changements: api.SHABChangements{SeatChanged: true, AddressChanged: true},
				},
			},
		},
	}
	if got := changeSummary(pub); got != "Mutation — seat, address" {
		t.Errorf("got %q", got)
	}
}

func TestChangeSummaryFallbackTitle(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Meta: api.SHABXMLMeta{
			Title: &api.SHABXMLTitle{DE: "Some very long German title that exceeds forty characters easily"},
		},
	}
	got := changeSummary(pub)
	if len(got) > 40 {
		t.Errorf("expected truncation to 40 chars, got %q (%d)", got, len(got))
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./cmd/ -run TestChangeSummary -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `cmd/shab_history.go`**

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

// changeSummary produces a short label for a publication based on its
// transaction kind, falling back to its title (truncated to 40 chars).
func changeSummary(pub *api.SHABPublicationXML) string {
	if pub == nil {
		return ""
	}
	if tx := pub.Content.Transaction; tx != nil {
		switch {
		case tx.Creation != nil:
			return "Neueintragung"
		case tx.Deletion != nil:
			return "Löschung"
		case tx.Update != nil:
			labels := tx.Update.Changements.ChangedLabels()
			if len(labels) == 0 {
				return "Mutation"
			}
			return "Mutation — " + strings.Join(labels, ", ")
		}
	}
	if pub.Meta.Title != nil {
		return output.Truncate(pub.Meta.Title.Pick(output.Lang), 40)
	}
	return ""
}

// historyEntry is one row of the timeline.
type historyEntry struct {
	Date              string `json:"date"`
	PublicationNumber string `json:"publicationNumber"`
	URL               string `json:"url"`
	ChangeSummary     string `json:"changeSummary"`
	IsCurrent         bool   `json:"isCurrent"`
	Unresolved        bool   `json:"unresolved,omitempty"`
}

var shabHistoryCmd = &cobra.Command{
	Use:   "history <number|uuid>",
	Short: "Show the chain of FOSC publications for the same legal entity",
	Long:  "Walks the lastFosc back-pointers from the given publication until there are no more prior FOSC entries.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		depth, _ := cmd.Flags().GetInt("depth")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		id, err := client.SHABResolveID(args[0])
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		chain, err := client.SHABHistory(id, depth)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(chain) == 0 {
			fmt.Println("No publication found.")
			return nil
		}

		entries := make([]historyEntry, 0, len(chain))
		// chain is newest → oldest from the walker; reverse for display
		for i := len(chain) - 1; i >= 0; i-- {
			pub := chain[i]
			e := historyEntry{
				Date:              pub.Meta.PublicationDate,
				PublicationNumber: pub.Meta.PublicationNumber,
				URL:               api.SHABPublicationURL(pub.Meta.ID),
				ChangeSummary:     changeSummary(pub),
				IsCurrent:         i == 0,
			}
			entries = append(entries, e)
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(entries)
			return nil
		}

		for _, e := range entries {
			marker := ""
			if e.IsCurrent {
				marker = "  ← current"
			}
			date := e.Date
			if len(date) >= 10 {
				date = date[:10]
			}
			fmt.Printf("%-10s  %-20s  %-40s  %s%s\n",
				date,
				e.PublicationNumber,
				output.Truncate(e.ChangeSummary, 40),
				output.Hyperlink(e.URL, "link"),
				marker,
			)
		}
		if len(chain) == 1 && (chain[0].Content.LastFosc == nil || chain[0].Content.LastFosc.Sequence == "") {
			fmt.Println("\nNo prior FOSC entries referenced by this publication.")
		}
		return nil
	},
}

func init() {
	shabHistoryCmd.Flags().Int("depth", 0, "Maximum number of back-hops (0 = unlimited)")
	shabCmd.AddCommand(shabHistoryCmd)
}
```

- [ ] **Step 4: Add `SHABHistory` to `api/shab.go`**

Append to `api/shab.go` (same package, so the return type is bare `*SHABPublicationXML`):

```go
// SHABHistory walks the lastFosc back-pointer chain starting from id and
// returns the publications newest-first. If depth > 0 the walk stops after
// that many hops. Unresolved references terminate the walk silently.
func (c *Client) SHABHistory(id string, depth int) ([]*SHABPublicationXML, error) {
	var chain []*SHABPublicationXML
	visited := map[string]bool{}
	curID := id
	hops := 0
	for curID != "" {
		if visited[curID] {
			break // guard against cycles
		}
		visited[curID] = true
		pub, _, err := c.SHABPublicationParsed(curID)
		if err != nil {
			if len(chain) == 0 {
				return nil, err
			}
			break // prior hop failed; stop but keep what we have
		}
		if pub == nil {
			break
		}
		chain = append(chain, pub)
		if pub.Content.LastFosc == nil || pub.Content.LastFosc.Sequence == "" {
			break
		}
		if depth > 0 && hops+1 >= depth {
			break
		}
		hops++
		next, err := c.SHABResolveID(pub.Content.LastFosc.Sequence)
		if err != nil || next == "" {
			break
		}
		curID = next
	}
	return chain, nil
}
```

- [ ] **Step 5: Run the change-summary tests**

Run: `go test ./cmd/ -run TestChangeSummary -v`
Expected: PASS.

- [ ] **Step 6: Build the whole project**

Run: `go build ./...`
Expected: success.

- [ ] **Step 7: Smoke test**

Run: `./chli shab history HR02-1006615899`
Expected: a list of dated rows oldest → newest, each with a `[link]`, the last marked `← current`. If there are no prior FOSC entries, you see the one-row fallback message.

Run: `./chli shab history HR02-1006615899 --json | head -40`
Expected: JSON array of `{date, publicationNumber, url, changeSummary, isCurrent}`.

- [ ] **Step 8: Commit**

```bash
git add api/shab.go cmd/shab_history.go cmd/shab_history_test.go
git commit -m "Add shab history command walking lastFosc chain"
```

---

## Task 8: Final verification

- [ ] **Step 1: Full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 2: Build**

Run: `go build -o chli .`
Expected: binary at `./chli`.

- [ ] **Step 3: End-to-end smoke**

Run each and eyeball the output:

```bash
./chli shab search "migros" --size 5
./chli shab publication HR02-1006615899
./chli shab publication HR02-1006615899 --diff
./chli shab history HR02-1006615899
./chli shab history HR02-1006615899 --json | head -20
```

Expected:
- Search: URL column present, last column is a clickable link when run in a TTY.
- Publication: structured HR block (Company/Seat/Address/Auditor/Changes) followed by the legal text.
- `--diff`: short before/after, or the "no field-level changes" note.
- History: dated rows oldest → newest with `← current` marker on the last.

- [ ] **Step 4: Commit the built binary if tracked**

Check: `git status chli`. The repo currently tracks `chli` (see `git log` on it). If the binary changed, commit:

```bash
git add chli
git commit -m "Rebuild chli with SHAB hyperlinks, richer detail, and history"
```

---

## Self-review notes

- Every type referenced in later tasks (`SHABCommons`, `SHABCompany`, `SHABAddress`, `SHABRevision`, `SHABRevisionCompany`, `SHABLastFosc`, `SHABTransaction`, `SHABTxUpdate`, `SHABChangements`, `SHABXMLTitle`, `SHABXMLOffice`) is defined in Task 3.
- `Hyperlink` (Task 1), `SHABPublicationURL` (Task 4), `diffCommons` / `printDiff` (Task 6), `changeSummary` / `SHABHistory` (Task 7) are each defined before first use.
- The spec's five numbered deliverables map to tasks: URL (Tasks 1, 4, 5) · richer XML (Task 3) · interactive render (Task 5) · diff (Task 6) · history (Task 7). Non-HR rubrics fall through the HR blocks naturally because each `if cn := ...; cn != nil` guard skips them.
- No placeholders, TODOs, or "fill in" steps.
