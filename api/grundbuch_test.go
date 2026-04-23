package api

import (
	"strings"
	"testing"
)

// The registry is the backbone of `chli grundbuch` — every field is surfaced
// to users via `canton` / `cantons` / `owner`. These invariants catch typos
// when new cantons are added and make silent regressions impossible.

func TestRegistryHasAll26Cantons(t *testing.T) {
	if got := len(Cantons); got != 26 {
		t.Fatalf("Cantons map has %d entries, want 26", got)
	}
	expected := []string{
		"AG", "AI", "AR", "BE", "BL", "BS", "FR", "GE", "GL", "GR",
		"JU", "LU", "NE", "NW", "OW", "SG", "SH", "SO", "SZ", "TG",
		"TI", "UR", "VD", "VS", "ZG", "ZH",
	}
	for _, code := range expected {
		if _, ok := Cantons[code]; !ok {
			t.Errorf("missing canton %s", code)
		}
	}
}

func TestRegistryFieldsAreSet(t *testing.T) {
	for code, c := range Cantons {
		t.Run(code, func(t *testing.T) {
			if c.Code != code {
				t.Errorf("Code=%q but key=%q", c.Code, code)
			}
			if c.Name["de"] == "" {
				t.Errorf("missing German name")
			}
			if c.Name["fr"] == "" {
				t.Errorf("missing French name")
			}
			if c.Tier == TierUnknown {
				t.Errorf("tier unset")
			}
			if c.ParcelPortal.URL == "" {
				t.Errorf("parcel portal URL unset")
			}
			if c.AuthModel == "" {
				t.Errorf("auth model unset")
			}
			if c.GrundbuchamtURL == "" {
				t.Errorf("Grundbuchamt URL unset")
			}
			if c.VerifiedAt == "" {
				t.Errorf("VerifiedAt unset — research date should be surfaced to users")
			}
			if c.LegalNotes == "" {
				t.Errorf("LegalNotes unset — every canton should carry a brief legal context line")
			}
			if len(c.OwnerOrder) == 0 {
				t.Errorf("OwnerOrder empty — every canton must have at least one official order channel")
			}
		})
	}
}

func TestRegistryTiersMatchOwnerEndpointPresence(t *testing.T) {
	// Tier T3/T4/T5 may still carry an OwnerPublic entry (documenting a free
	// viewer that's gated by SMS or federal eID) — that's the honest shape of
	// the data. Only check that T1 / T2 always have an OwnerPublic since
	// that's what those tiers *mean*.
	for code, c := range Cantons {
		if (c.Tier == TierT1 || c.Tier == TierT2) && c.OwnerPublic == nil {
			t.Errorf("%s: tier %s implies a public owner endpoint but none is set", code, c.Tier)
		}
	}
}

func TestNormalizeEGRID(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"canonical", "CH294676423526", "CH294676423526", false},
		{"lowercase", "ch294676423526", "CH294676423526", false},
		{"spaces", "CH 2946 7642 3526", "CH294676423526", false},
		{"12-digit variant", "CH123456789012", "CH123456789012", false},
		{"10-digit variant", "CH1234567890", "CH1234567890", false},
		{"empty", "", "", true},
		{"missing CH prefix", "294676423526", "", true},
		{"too short", "CH123", "", true},
		{"non-digits", "CH294676XYZ526", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeEGRID(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCantonCode(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"ZH", "ZH", false},
		{"zh", "ZH", false},
		{" be ", "BE", false},
		{"XX", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := CantonCode(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.in)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSearchDetail(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantMuni string
		wantBFS  int
		wantCant string
	}{
		{
			name:     "address Bern",
			in:       "bundesplatz 3 3011 bern 351 bern ch be",
			wantMuni: "Bern",
			wantBFS:  351,
			wantCant: "BE",
		},
		{
			name:     "address Geneva",
			in:       "rue du rhone 1 1204 geneve 6621 geneve ch ge",
			wantMuni: "Geneve",
			wantBFS:  6621,
			wantCant: "GE",
		},
		{
			name:     "parcel shape",
			in:       "823 351 bern ch be",
			wantMuni: "Bern",
			wantBFS:  351,
			wantCant: "BE",
		},
		{
			name:     "hyphenated muni",
			in:       "rue de la gare 1 2300 la-chaux-de-fonds 6421 la-chaux-de-fonds ch ne",
			wantMuni: "La-Chaux-De-Fonds",
			wantBFS:  6421,
			wantCant: "NE",
		},
		{
			name:     "empty",
			in:       "",
			wantMuni: "",
			wantBFS:  0,
			wantCant: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			muni, bfs, canton := parseSearchDetail(tt.in)
			if muni != tt.wantMuni {
				t.Errorf("muni=%q want %q", muni, tt.wantMuni)
			}
			if bfs != tt.wantBFS {
				t.Errorf("bfs=%d want %d", bfs, tt.wantBFS)
			}
			if canton != tt.wantCant {
				t.Errorf("canton=%q want %q", canton, tt.wantCant)
			}
		})
	}
}

func TestOrderedCantonsIsSorted(t *testing.T) {
	list := OrderedCantons()
	if len(list) != 26 {
		t.Fatalf("OrderedCantons returned %d entries, want 26", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].Code >= list[i].Code {
			t.Errorf("not sorted: %s >= %s at index %d", list[i-1].Code, list[i].Code, i)
		}
	}
}

func TestLocalizedName(t *testing.T) {
	zh := Cantons["ZH"]
	if got := zh.LocalizedName("de"); got != "Zürich" {
		t.Errorf("LocalizedName(de) = %q, want Zürich", got)
	}
	if got := zh.LocalizedName("fr"); got != "Zurich" {
		t.Errorf("LocalizedName(fr) = %q, want Zurich", got)
	}
	// Unknown language falls through to de.
	if got := zh.LocalizedName("xx"); got != "Zürich" {
		t.Errorf("LocalizedName(xx) = %q, want fallback Zürich", got)
	}
}

func TestValidCantonListIsStable(t *testing.T) {
	// Error messages include the list of valid cantons — ensure the order is
	// deterministic so tests of those error paths are not flaky.
	a := validCantonList()
	b := validCantonList()
	if a != b {
		t.Errorf("validCantonList is non-deterministic")
	}
	if !strings.HasPrefix(a, "AG,") {
		t.Errorf("expected list to start with AG, got %q", a[:10])
	}
}
