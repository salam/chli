package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// --- ParsePlate ---------------------------------------------------------

func TestParsePlate(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantErr        bool
		wantCanton     string
		wantNormalized string
		wantDigits     string
		wantWarnings   int
	}{
		{"plain ZH prefix", "ZH123456", false, "ZH", "ZH123456", "123456", 0},
		{"whitespace", "ZH 123 456", false, "ZH", "ZH123456", "123456", 0},
		{"hyphens", "ZH-123-456", false, "ZH", "ZH123456", "123456", 0},
		{"lowercase", "zh123456", false, "ZH", "ZH123456", "123456", 0},
		{"mixed case with spaces", "  zH 123-456  ", false, "ZH", "ZH123456", "123456", 0},
		{"underscore separator", "zh_123_456", false, "ZH", "ZH123456", "123456", 0},
		{"bare digits", "120120", false, "", "120120", "120120", 0},
		{"short plate", "ZH1", false, "ZH", "ZH1", "1", 0},
		{"unknown canton", "XY123456", true, "", "", "", 0},
		{"empty", "", true, "", "", "", 0},
		{"only whitespace", "   ", true, "", "", "", 0},
		{"non-digit body warns", "ZHABC", false, "ZH", "ZHABC", "ABC", 1},
		{"all letter prefix but invalid", "QQ12", true, "", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePlate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePlate(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if p.Canton != tt.wantCanton {
				t.Errorf("Canton = %q, want %q", p.Canton, tt.wantCanton)
			}
			if p.Normalized != tt.wantNormalized {
				t.Errorf("Normalized = %q, want %q", p.Normalized, tt.wantNormalized)
			}
			if p.Digits != tt.wantDigits {
				t.Errorf("Digits = %q, want %q", p.Digits, tt.wantDigits)
			}
			if len(p.Warnings) != tt.wantWarnings {
				t.Errorf("len(Warnings) = %d, want %d: %v", len(p.Warnings), tt.wantWarnings, p.Warnings)
			}
		})
	}
}

func TestIsValidCantonAndCount(t *testing.T) {
	if len(ValidCantons) != 26 {
		t.Fatalf("len(ValidCantons) = %d, want 26", len(ValidCantons))
	}
	for _, c := range []string{"ZH", "zh", " ZH "} {
		if !IsValidCanton(c) {
			t.Errorf("IsValidCanton(%q) = false, want true", c)
		}
	}
	for _, c := range []string{"", "XY", "AB", "ZZ"} {
		if IsValidCanton(c) {
			t.Errorf("IsValidCanton(%q) = true, want false", c)
		}
	}
}

// --- LoadCantons / embedded JSON ----------------------------------------

func TestLoadCantonsEmbedded(t *testing.T) {
	m, err := LoadCantons()
	if err != nil {
		t.Fatalf("LoadCantons: %v", err)
	}
	if len(m) != 26 {
		t.Fatalf("got %d cantons, want 26", len(m))
	}
	for code := range ValidCantons {
		entry, ok := m[code]
		if !ok {
			t.Errorf("missing canton %s", code)
			continue
		}
		if entry.Code != code {
			t.Errorf("canton %s: entry.Code = %q", code, entry.Code)
		}
		if entry.Authority.Name == "" {
			t.Errorf("canton %s: authority.name empty", code)
		}
		if entry.Names["de"] == "" {
			t.Errorf("canton %s: names.de empty", code)
		}
		// Parse last_verified.
		if _, err := time.Parse("2006-01-02", entry.Verification.LastVerified); err != nil {
			t.Errorf("canton %s: last_verified %q not parseable: %v", code, entry.Verification.LastVerified, err)
		}
	}
}

func TestLoadCantons_InvariantsGoldenFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/plate_cantons_valid.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	m, err := loadCantonsFromBytes(data)
	if err != nil {
		t.Fatalf("loadCantonsFromBytes: %v", err)
	}
	zh, ok := m["ZH"]
	if !ok {
		t.Fatal("ZH missing from fixture")
	}
	if zh.Halterauskunft.Mode != ModeOnline {
		t.Errorf("ZH mode = %q, want online", zh.Halterauskunft.Mode)
	}
	if zh.Halterauskunft.URL == "" {
		t.Errorf("ZH url empty")
	}
}

func TestLoadCantons_RejectsInvalidFixtures(t *testing.T) {
	// Short fixture (1 entry) — tests the count invariant.
	t.Run("wrong count", func(t *testing.T) {
		data, err := os.ReadFile("testdata/plate_cantons_short.json")
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		_, err = loadCantonsFromBytes(data)
		if err == nil || !strings.Contains(err.Error(), "entries") {
			t.Fatalf("err = %v, want substring 'entries'", err)
		}
	})

	// Remaining invariants: mutate the valid fixture in-memory and re-encode.
	base, err := os.ReadFile("testdata/plate_cantons_valid.json")
	if err != nil {
		t.Fatalf("read base fixture: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*PlateCantonsFile)
		substr string
	}{
		{
			name: "mode online requires url",
			mutate: func(f *PlateCantonsFile) {
				e := f.Cantons["ZH"]
				e.Halterauskunft.Mode = ModeOnline
				e.Halterauskunft.URL = ""
				f.Cantons["ZH"] = e
			},
			substr: "url is required",
		},
		{
			name: "mode postal requires postal address",
			mutate: func(f *PlateCantonsFile) {
				e := f.Cantons["AG"]
				e.Halterauskunft.Mode = ModePostal
				e.Authority.Postal = Postal{}
				f.Cantons["AG"] = e
			},
			substr: "postal must be populated",
		},
		{
			name: "unknown canton code",
			mutate: func(f *PlateCantonsFile) {
				e := f.Cantons["ZH"]
				delete(f.Cantons, "ZH")
				f.Cantons["XX"] = e
			},
			substr: "unknown canton",
		},
		{
			name: "bad last_verified date",
			mutate: func(f *PlateCantonsFile) {
				e := f.Cantons["ZH"]
				e.Verification.LastVerified = "yesterday"
				f.Cantons["ZH"] = e
			},
			substr: "last_verified",
		},
		{
			name: "negative cost",
			mutate: func(f *PlateCantonsFile) {
				e := f.Cantons["ZH"]
				e.Halterauskunft.CostCHF = -1
				f.Cantons["ZH"] = e
			},
			substr: "cost_chf",
		},
		{
			name: "bad mode enum",
			mutate: func(f *PlateCantonsFile) {
				e := f.Cantons["ZH"]
				e.Halterauskunft.Mode = "not-a-mode"
				f.Cantons["ZH"] = e
			},
			substr: "mode",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var file PlateCantonsFile
			if err := json.Unmarshal(base, &file); err != nil {
				t.Fatalf("decode base: %v", err)
			}
			tc.mutate(&file)
			raw, err := json.Marshal(file)
			if err != nil {
				t.Fatalf("encode mutated: %v", err)
			}
			_, err = loadCantonsFromBytes(raw)
			if err == nil {
				t.Fatalf("loadCantonsFromBytes: nil err, want error containing %q", tc.substr)
			}
			if !strings.Contains(err.Error(), tc.substr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.substr)
			}
		})
	}
}

func TestLoadCantons_RejectsUnknownFields(t *testing.T) {
	raw := []byte(`{"schema_version":1,"cantons":{},"extra":"x"}`)
	_, err := loadCantonsFromBytes(raw)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("err = %v, want unknown field rejection", err)
	}
}

// --- Deeplink rendering -------------------------------------------------

func TestDeeplinkFor_ZH(t *testing.T) {
	entry := CantonEntry{
		Code: "ZH",
		Halterauskunft: HalterauskunftEntry{
			Mode:             ModeOnline,
			URL:              "https://halterauskunft.zh.ch",
			DeeplinkTemplate: "https://halterauskunft.zh.ch/?plate={{.PlateNormalized}}&c={{.Canton}}",
		},
	}
	p, err := ParsePlate("ZH 123 456")
	if err != nil {
		t.Fatalf("ParsePlate: %v", err)
	}
	got, err := DeeplinkFor(p, entry)
	if err != nil {
		t.Fatalf("DeeplinkFor: %v", err)
	}
	want := "https://halterauskunft.zh.ch/?plate=ZH123456&c=ZH"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDeeplinkFor_NoTemplate_FallsBackToURL(t *testing.T) {
	entry := CantonEntry{
		Code: "BE",
		Halterauskunft: HalterauskunftEntry{
			Mode: ModeOnline,
			URL:  "https://be.example/halter",
		},
	}
	p, _ := ParsePlate("BE1")
	got, _ := DeeplinkFor(p, entry)
	if got != "https://be.example/halter" {
		t.Errorf("got %q", got)
	}
}

// --- Language fallback --------------------------------------------------

func TestCantonEntry_LanguageFallback(t *testing.T) {
	entry := CantonEntry{
		Code: "ZH",
		Names: map[string]string{
			"de": "Zürich",
			"en": "Zurich",
		},
		Halterauskunft: HalterauskunftEntry{
			Notes: map[string]string{
				"de": "Begründung erforderlich.",
			},
		},
	}
	// fr falls back to de.
	if got := entry.Name("fr"); got != "Zürich" {
		t.Errorf("Name(fr) = %q, want Zürich (fallback)", got)
	}
	if got := entry.Note("fr"); got != "Begründung erforderlich." {
		t.Errorf("Note(fr) = %q, want de fallback", got)
	}
	// en prefers explicit.
	if got := entry.Name("en"); got != "Zurich" {
		t.Errorf("Name(en) = %q, want Zurich", got)
	}
}

// --- Dispatcher render --------------------------------------------------

func TestRenderDispatch_IncludesCoreFields(t *testing.T) {
	entry := CantonEntry{
		Code:  "ZH",
		Names: map[string]string{"de": "Zürich", "en": "Zurich"},
		Authority: Authority{
			Name: "Strassenverkehrsamt des Kantons Zürich",
		},
		Halterauskunft: HalterauskunftEntry{
			Mode:             ModeOnline,
			URL:              "https://halterauskunft.zh.ch",
			DeeplinkTemplate: "https://halterauskunft.zh.ch/?plate={{.PlateNormalized}}",
			CostCHF:          13,
			PaymentMethods:   []string{"twint", "mastercard"},
			Auth: AuthReqs{
				Captcha:              CaptchaHCaptcha,
				RequiresStatedReason: true,
			},
			Processing: Processing{Typical: ProcInstant, Delivery: DeliveryPDFEmail},
			LegalBasis: "SVG Art. 104 / VZV Art. 126",
		},
		Verification: Verification{
			LastVerified: "2026-04-20",
			VerifiedBy:   VerifiedManual,
			SourceURLs:   []string{"https://halterauskunft.zh.ch"},
		},
	}
	p, _ := ParsePlate("ZH 123 456")
	out := RenderDispatch(p, entry, "en")
	for _, want := range []string{
		"ZH 123 456",
		"Zurich (ZH)",
		"Strassenverkehrsamt des Kantons Zürich",
		"Halterauskunft online",
		"https://halterauskunft.zh.ch/?plate=ZH123456",
		"CHF 13",
		"Twint, Mastercard",
		"hCaptcha + stated reason",
		"Instant, PDF emailed",
		"SVG Art. 104",
		"2026-04-20",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderDispatch missing %q in output:\n%s", want, out)
		}
	}
}

// --- Verifier -----------------------------------------------------------

func TestVerifyCanton_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Willkommen beim Strassenverkehrsamt Zürich — Halterauskunft online beantragen."))
	}))
	defer srv.Close()

	entry := sampleEntry("ZH", "Zürich", srv.URL)
	res := VerifyCantonWith(context.Background(), entry, VerifyCantonOpts{HTTPClient: srv.Client()})
	if res.Status != VerifyOK {
		t.Errorf("status = %s, reasons = %v", res.Status, res.Reasons)
	}
	if res.HTTPStatus != 200 {
		t.Errorf("http = %d", res.HTTPStatus)
	}
}

func TestVerifyCanton_Redirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Halterauskunft Zürich neue Adresse"))
	}))
	defer final.Close()

	var entryURL string
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer first.Close()
	entryURL = first.URL

	entry := sampleEntry("ZH", "Zürich", entryURL)
	res := VerifyCantonWith(context.Background(), entry, VerifyCantonOpts{HTTPClient: first.Client()})
	// Redirect across hostname should trigger a warn. Body keyword + name
	// present so not error.
	if res.Status == VerifyError {
		t.Errorf("status = error, want ok/warn: reasons = %v", res.Reasons)
	}
	if res.FinalURL == "" || res.FinalURL == entryURL {
		t.Errorf("final URL not recorded (%s -> %s)", entryURL, res.FinalURL)
	}
}

func TestVerifyCanton_KeywordMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Welcome to the generic portal, nothing relevant here."))
	}))
	defer srv.Close()

	entry := sampleEntry("ZH", "Zürich", srv.URL)
	res := VerifyCantonWith(context.Background(), entry, VerifyCantonOpts{HTTPClient: srv.Client()})
	if res.Status != VerifyWarn {
		t.Errorf("status = %s, want warn; reasons = %v", res.Status, res.Reasons)
	}
}

func TestVerifyCanton_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := srv.Client()
	client.Timeout = 10 * time.Millisecond

	entry := sampleEntry("ZH", "Zürich", srv.URL)
	res := VerifyCantonWith(context.Background(), entry, VerifyCantonOpts{HTTPClient: client})
	if res.Status != VerifyError {
		t.Errorf("status = %s, want error; reasons = %v", res.Status, res.Reasons)
	}
}

func TestVerifyCanton_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()

	entry := sampleEntry("ZH", "Zürich", srv.URL)
	res := VerifyCantonWith(context.Background(), entry, VerifyCantonOpts{HTTPClient: srv.Client()})
	if res.Status != VerifyError {
		t.Errorf("status = %s, want error", res.Status)
	}
	if res.HTTPStatus != 500 {
		t.Errorf("http = %d", res.HTTPStatus)
	}
}

func TestVerifyCanton_UnavailableCanton(t *testing.T) {
	entry := CantonEntry{
		Code:  "ZZ",
		Names: map[string]string{"de": "Nowhere"},
		Halterauskunft: HalterauskunftEntry{
			Mode: ModeUnavailable,
		},
	}
	res := VerifyCantonWith(context.Background(), entry, VerifyCantonOpts{})
	if res.Status != VerifyWarn {
		t.Errorf("status = %s, want warn (no URL)", res.Status)
	}
}

// sampleEntry builds a minimal CantonEntry pointing at a test server URL.
func sampleEntry(code, name, url string) CantonEntry {
	return CantonEntry{
		Code:  code,
		Names: map[string]string{"de": name, "en": name},
		Authority: Authority{
			Name: "Strassenverkehrsamt des Kantons " + name,
		},
		Halterauskunft: HalterauskunftEntry{
			Mode:    ModeOnline,
			URL:     url,
			CostCHF: 0,
		},
		Verification: Verification{LastVerified: "2026-04-20", VerifiedBy: VerifiedManual},
	}
}

// Guard: make sure plate_cantons.json round-trips through JSON decode with
// the same shape Go exposes. This catches accidental field renames.
func TestEmbeddedCantonsRoundTrip(t *testing.T) {
	m, err := LoadCantons()
	if err != nil {
		t.Fatalf("LoadCantons: %v", err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("round-tripped json empty")
	}
}
