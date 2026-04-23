package api

import "encoding/json"

// Tier classifies what a canton offers for unauthenticated owner lookup.
//
//	T1 — fully public owner via a canton endpoint (viewer or API).
//	T2 — semi-public / rolling-out owner lookup.
//	T3 — free but gated (SMS to Swiss mobile, daily quota) — not automatable.
//	T4 — free but requires a full eID (AGOV / SwissID).
//	T5 — no public owner; requires Terravis/Intercapi, counter visit, or paid order.
//
// Parcel/geometry/EGRID is universally available via the federal aggregated
// cadastre regardless of tier.
type Tier int

const (
	TierUnknown Tier = iota
	TierT1
	TierT2
	TierT3
	TierT4
	TierT5
)

func (t Tier) String() string {
	switch t {
	case TierT1:
		return "T1"
	case TierT2:
		return "T2"
	case TierT3:
		return "T3"
	case TierT4:
		return "T4"
	case TierT5:
		return "T5"
	default:
		return "?"
	}
}

// AuthModel describes how owner data is gated for this canton. Used only to
// tell the user what to expect — the CLI does not perform any of these flows.
type AuthModel string

const (
	AuthNone         AuthModel = "none"
	AuthSMSPhone     AuthModel = "sms-to-ch-mobile"
	AuthAGOV         AuthModel = "agov"
	AuthSwissID      AuthModel = "swissid"
	AuthProfessional AuthModel = "professional-convention"
	AuthCounter      AuthModel = "counter-or-mail"
)

// PortalRef is a named URL with a protocol/type hint.
type PortalRef struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type,omitempty"` // wms | wfs | rest | viewer | form | email | office
}

// CostSpec expresses what an owner extract costs. Either Fixed, a Min/Max
// range, or Unpriced with a verification URL. The Notes field holds extras
// like postage or certification supplements.
type CostSpec struct {
	FixedCHF *int   `json:"fixed_chf,omitempty"`
	MinCHF   *int   `json:"min_chf,omitempty"`
	MaxCHF   *int   `json:"max_chf,omitempty"`
	Unpriced bool   `json:"unpriced,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// OwnerEndpoint describes a canton's public owner lookup endpoint, when one
// exists. Verified means endpoint probing confirmed owner data returns
// server-side (not only via JS rendering). In Phase 1 all OwnerEndpoints are
// informational — the CLI does not yet call them.
type OwnerEndpoint struct {
	URL      string `json:"url"`
	Type     string `json:"type"`               // rest | wfs-getfeatureinfo | wms-getfeatureinfo | viewer
	Verified bool   `json:"verified,omitempty"` // probing confirmed server-side owner attribute
	Notes    string `json:"notes,omitempty"`
}

// CantonCapability is one row of the static per-canton registry.
type CantonCapability struct {
	Code            string            `json:"code"`
	Name            map[string]string `json:"name"` // de/fr/it/en
	Tier            Tier              `json:"tier"`
	ParcelPortal    PortalRef         `json:"parcel_portal"`
	OwnerPublic     *OwnerEndpoint    `json:"owner_public,omitempty"`
	OwnerOrder      []PortalRef       `json:"owner_order"`
	AuthModel       AuthModel         `json:"auth_model"`
	Cost            CostSpec          `json:"cost"`
	GrundbuchamtURL string            `json:"grundbuchamt_url"`
	LegalNotes      string            `json:"legal_notes,omitempty"`
	Caveats         []string          `json:"caveats,omitempty"`
	VerifiedAt      string            `json:"verified_at"`
}

// LocalizedName picks a display name by language, falling back to de → fr → en.
func (c CantonCapability) LocalizedName(lang string) string {
	if n, ok := c.Name[lang]; ok && n != "" {
		return n
	}
	for _, k := range []string{"de", "fr", "it", "en"} {
		if n, ok := c.Name[k]; ok && n != "" {
			return n
		}
	}
	return c.Code
}

// Parcel is the normalized result of a parcel lookup across any input type.
type Parcel struct {
	EGRID        string          `json:"egrid"`
	Canton       string          `json:"canton"`
	Municipality string          `json:"municipality,omitempty"`
	BFS          int             `json:"bfs,omitempty"`
	Number       string          `json:"number,omitempty"` // canton-local parcel number as a string (may include letters, slashes)
	AreaM2       int             `json:"area_m2,omitempty"`
	LV95E        float64         `json:"lv95_e,omitempty"`
	LV95N        float64         `json:"lv95_n,omitempty"`
	Lat          float64         `json:"lat,omitempty"`
	Lon          float64         `json:"lon,omitempty"`
	Portal       string          `json:"portal,omitempty"`
	Geometry     json.RawMessage `json:"geometry,omitempty"` // GeoJSON when --json is set and server returned it
	Source       string          `json:"source,omitempty"`   // api3.geo.admin.ch, geodienste.ch, etc.
}

// Owner is the normalized result of an owner lookup. In Phase 1 this is
// unused (no live adapters); the struct is defined here so capability-only
// JSON output can surface it as null consistently.
type Owner struct {
	Names        []string `json:"names"`
	Ownership    string   `json:"ownership,omitempty"`   // e.g. "Miteigentum je 1/2"
	Acquired     string   `json:"acquired,omitempty"`    // ISO date or year
	Source       string   `json:"source"`                // which canton portal the data came from
	Disclaimer   string   `json:"disclaimer"`            // always "unofficial; no legal validity"
	Suppressed   bool     `json:"suppressed,omitempty"`  // canton reported owner suppressed on request
	RetrievedAt  string   `json:"retrieved_at"`
}
