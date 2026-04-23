package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	grundbuchCacheTTL    = 7 * 24 * time.Hour // parcel geometry / EGRID: 7 days
	grundbuchParcelLayer = "ch.kantone.cadastralwebmap-farbe"
)

// egridRE matches an EGRID: the letters CH followed by 10-16 digits. The federal
// Leitfaden does not fix a single length (allocations vary by Grundbuchkreis),
// so this is intentionally permissive. Validation rejects anything else.
var egridRE = regexp.MustCompile(`^CH[0-9]{10,16}$`)

// NormalizeEGRID uppercases and strips internal spaces, then validates shape.
func NormalizeEGRID(raw string) (string, error) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
	if !egridRE.MatchString(s) {
		return "", fmt.Errorf("invalid EGRID %q: expected 'CH' followed by 10-16 digits", raw)
	}
	return s, nil
}

// CantonCode validates a 2-letter canton code against the capability registry.
func CantonCode(raw string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if _, ok := Cantons[s]; !ok {
		return "", fmt.Errorf("unknown canton %q (valid: %s)", raw, validCantonList())
	}
	return s, nil
}

func validCantonList() string {
	codes := make([]string, 0, len(Cantons))
	for c := range Cantons {
		codes = append(codes, c)
	}
	// Sort for stable error messages.
	for i := range codes {
		for j := i + 1; j < len(codes); j++ {
			if codes[j] < codes[i] {
				codes[i], codes[j] = codes[j], codes[i]
			}
		}
	}
	return strings.Join(codes, ", ")
}

// GrundbuchSearchAddress searches api3.geo.admin.ch for addresses/parcels
// matching the query. Returns candidate hits with LV95 coordinates and
// (where derivable) municipality / BFS / canton parsed from the detail field.
func (c *Client) GrundbuchSearchAddress(query string) ([]GrundbuchHit, error) {
	q := url.Values{}
	q.Set("searchText", query)
	q.Set("type", "locations")
	q.Set("origins", "address,parcel,gg25")
	q.Set("sr", "2056") // LV95
	q.Set("limit", "10")

	var out GeoSearchResponse
	if err := c.DoJSONWithTTL(geoBaseURL, "/rest/services/api/SearchServer?"+q.Encode(), &out, grundbuchCacheTTL); err != nil {
		return nil, fmt.Errorf("geo.admin.ch SearchServer: %w", err)
	}
	hits := make([]GrundbuchHit, 0, len(out.Results))
	for _, r := range out.Results {
		h := GrundbuchHit{
			Label:  stripHTMLTags(r.Attrs.Label),
			Detail: r.Attrs.Detail,
			Layer:  r.Attrs.Layer,
			Origin: r.Attrs.Origin,
			LV95E:  r.Attrs.Y, // SearchServer returns LV95 as (y=east, x=north) — swisstopo convention
			LV95N:  r.Attrs.X,
		}
		h.Municipality, h.BFS, h.Canton = parseSearchDetail(r.Attrs.Detail)
		hits = append(hits, h)
	}
	return hits, nil
}

// parseSearchDetail extracts municipality, BFS number, and canton from the
// space-separated detail string SearchServer returns for address/parcel hits.
//
// Detail format (confirmed empirically, lowercase, space-separated):
//
//	address: "<street> <no> <plz> <ortschaft> <bfs> <gemeinde> ch <kt>"
//	parcel:  "<parcel#> <bfs> <gemeinde> ch <kt>"
//
// The last four tokens are always "<bfs> <gemeinde> ch <kt>", which lets us
// read them positionally without parsing the variable-length prefix.
func parseSearchDetail(s string) (muni string, bfs int, canton string) {
	if s == "" {
		return
	}
	tokens := strings.Fields(s)
	n := len(tokens)
	if n < 4 {
		return
	}
	if tokens[n-2] != "ch" {
		return
	}
	last := tokens[n-1]
	if len(last) != 2 {
		return
	}
	canton = strings.ToUpper(last)

	muniRaw := tokens[n-3]
	if v, err := strconv.Atoi(tokens[n-4]); err == nil && v > 0 && v < 10000 {
		bfs = v
	}
	muni = titleCase(muniRaw)
	return
}

// titleCase upper-cases the first letter of each whitespace-separated word
// in a lowercase Gemeinde name (e.g. "la chaux-de-fonds" → "La Chaux-De-Fonds").
// strings.Title is deprecated; we don't need Unicode-complete handling here
// because SearchServer already lowercases its detail tokens.
func titleCase(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	nextUpper := true
	for _, r := range s {
		switch r {
		case ' ', '-':
			nextUpper = true
			b.WriteRune(r)
		default:
			if nextUpper {
				b.WriteRune(runeToUpper(r))
				nextUpper = false
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func runeToUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// GrundbuchIdentifyByCoord identifies the parcel at an LV95 coordinate using
// the federated cadastral webmap layer. Returns EGRID, parcel number, canton,
// and the canton's parcel portal URL. Municipality/BFS are best-effort — the
// cadastral layer itself does not expose them; callers that have a search hit
// in hand should use GrundbuchIdentifyByHit to preserve muni/BFS from the
// search phase.
func (c *Client) GrundbuchIdentifyByCoord(lv95E, lv95N float64) (*Parcel, error) {
	q := url.Values{}
	q.Set("geometry", fmt.Sprintf("%f,%f", lv95E, lv95N))
	q.Set("geometryType", "esriGeometryPoint")
	q.Set("sr", "2056")
	q.Set("mapExtent", fmt.Sprintf("%f,%f,%f,%f", lv95E-10, lv95N-10, lv95E+10, lv95N+10))
	q.Set("imageDisplay", "500,500,96")
	q.Set("tolerance", "0")
	q.Set("layers", "all:"+grundbuchParcelLayer)
	q.Set("returnGeometry", "false")

	var out GeoIdentifyResponse
	if err := c.DoJSONWithTTL(geoBaseURL, "/rest/services/api/MapServer/identify?"+q.Encode(), &out, grundbuchCacheTTL); err != nil {
		return nil, fmt.Errorf("geo.admin.ch identify: %w", err)
	}
	return parcelFromIdentify(out, lv95E, lv95N)
}

// GrundbuchIdentifyByHit identifies the parcel at a search hit and enriches
// the result with municipality / BFS / canton parsed from the search detail.
func (c *Client) GrundbuchIdentifyByHit(hit GrundbuchHit) (*Parcel, error) {
	p, err := c.GrundbuchIdentifyByCoord(hit.LV95E, hit.LV95N)
	if err != nil {
		return nil, err
	}
	if p.Municipality == "" && hit.Municipality != "" {
		p.Municipality = hit.Municipality
	}
	if p.BFS == 0 && hit.BFS != 0 {
		p.BFS = hit.BFS
	}
	if p.Canton == "" && hit.Canton != "" {
		p.Canton = hit.Canton
	}
	if p.Portal == "" && p.Canton != "" {
		if cap, ok := Cantons[p.Canton]; ok {
			p.Portal = cap.ParcelPortal.URL
		}
	}
	return p, nil
}

// GrundbuchFindByEGRID resolves a parcel by its EGRID via the federated layer.
// The find endpoint returns canton (ak), parcel number, and the canton's
// geoportal URL; it does not return geometry or municipality. Coordinate and
// municipality can be obtained by a follow-up feature fetch — deferred to
// Phase 2.
func (c *Client) GrundbuchFindByEGRID(egrid string) (*Parcel, error) {
	egrid, err := NormalizeEGRID(egrid)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("layer", grundbuchParcelLayer)
	q.Set("searchField", "egris_egrid")
	q.Set("searchText", egrid)
	q.Set("returnGeometry", "false")

	var out GeoIdentifyResponse
	if err := c.DoJSONWithTTL(geoBaseURL, "/rest/services/api/MapServer/find?"+q.Encode(), &out, grundbuchCacheTTL); err != nil {
		return nil, fmt.Errorf("geo.admin.ch find by EGRID: %w", err)
	}
	if len(out.Results) == 0 {
		return nil, fmt.Errorf("no parcel found for EGRID %s", egrid)
	}
	p := buildParcel(out.Results[0])
	if p.EGRID == "" {
		p.EGRID = egrid
	}
	// Enrich with the feature bbox (→ centroid coord) via the feature endpoint.
	if fid := findFeatureID(out.Results[0]); fid != "" {
		if e, n, ok := c.fetchParcelCentroid(fid); ok {
			p.LV95E, p.LV95N = e, n
		}
	}
	if p.Portal == "" && p.Canton != "" {
		if cap, ok := Cantons[p.Canton]; ok {
			p.Portal = cap.ParcelPortal.URL
		}
	}
	return p, nil
}

// fetchParcelCentroid calls the single-feature endpoint to read the bbox and
// derive a rough centroid in LV95. Returns ok=false on any error.
func (c *Client) fetchParcelCentroid(featureID string) (float64, float64, bool) {
	path := "/rest/services/api/MapServer/" + grundbuchParcelLayer + "/" + featureID + "?sr=2056"
	var wrap struct {
		Feature struct {
			Bbox []float64 `json:"bbox"`
		} `json:"feature"`
	}
	if err := c.DoJSONWithTTL(geoBaseURL, path, &wrap, grundbuchCacheTTL); err != nil {
		return 0, 0, false
	}
	if len(wrap.Feature.Bbox) != 4 {
		return 0, 0, false
	}
	e := (wrap.Feature.Bbox[0] + wrap.Feature.Bbox[2]) / 2
	n := (wrap.Feature.Bbox[1] + wrap.Feature.Bbox[3]) / 2
	return e, n, true
}

func findFeatureID(r GeoIdentifyResult) string {
	if len(r.FeatureID) > 0 {
		s := strings.Trim(string(r.FeatureID), `"`)
		if s != "" && s != "null" {
			return s
		}
	}
	return ""
}

// parcelFromIdentify extracts the parcel feature from an identify response on
// the cadastral webmap layer. Canton and portal URL come from the layer's
// attributes (`ak` and `geoportal_url`). Municipality/BFS are not exposed by
// this layer and must be added by the caller via GrundbuchIdentifyByHit.
func parcelFromIdentify(resp GeoIdentifyResponse, lv95E, lv95N float64) (*Parcel, error) {
	for i := range resp.Results {
		r := &resp.Results[i]
		if r.LayerBodID == grundbuchParcelLayer {
			p := buildParcel(*r)
			if p.LV95E == 0 && p.LV95N == 0 {
				p.LV95E = lv95E
				p.LV95N = lv95N
			}
			if p.Portal == "" && p.Canton != "" {
				if cap, ok := Cantons[p.Canton]; ok {
					p.Portal = cap.ParcelPortal.URL
				}
			}
			return p, nil
		}
	}
	return nil, fmt.Errorf("no parcel at coordinate (LV95 %.0f,%.0f)", lv95E, lv95N)
}

// buildParcel extracts the normalized parcel shape from a single identify
// result on the cadastral layer.
func buildParcel(r GeoIdentifyResult) *Parcel {
	p := &Parcel{
		Source: "api3.geo.admin.ch",
	}
	if v := firstString(r.Attributes, "egris_egrid", "egrid"); v != "" {
		p.EGRID = strings.ToUpper(v)
	}
	if v := firstString(r.Attributes, "number", "nummer", "parcel_number", "nbident_parcel", "egris_number"); v != "" {
		p.Number = v
	}
	if a := firstInt(r.Attributes, "flaeche_m2", "area", "area_m2"); a > 0 {
		p.AreaM2 = a
	}
	if v := firstString(r.Attributes, "gemeinde", "gemeindename", "commune", "municipality"); v != "" {
		p.Municipality = v
	}
	if v := firstInt(r.Attributes, "bfs_nummer", "bfs_num", "bfs"); v > 0 {
		p.BFS = v
	}
	// Parcel layer uses "ak" (Amtskürzel) for the 2-letter canton code.
	if v := firstString(r.Attributes, "ak", "kanton", "kanton_kuerzel"); v != "" {
		p.Canton = strings.ToUpper(v)
	}
	// Parcel layer exposes its canton-specific geoportal link as "geoportal_url".
	if v := firstString(r.Attributes, "geoportal_url"); v != "" {
		p.Portal = v
	}
	// Centroid / label point — identify returns it as x/y in the requested SR.
	if x, ok := floatAttr(r.Attributes, "geom_st_x_centroid", "x"); ok {
		p.LV95E = x
	}
	if y, ok := floatAttr(r.Attributes, "geom_st_y_centroid", "y"); ok {
		p.LV95N = y
	}
	return p
}

// OrderedCantons returns the registry keys sorted alphabetically — used for
// deterministic output of the `cantons` command.
func OrderedCantons() []CantonCapability {
	codes := make([]string, 0, len(Cantons))
	for c := range Cantons {
		codes = append(codes, c)
	}
	// Insertion sort for the 26-entry list.
	for i := range codes {
		for j := i + 1; j < len(codes); j++ {
			if codes[j] < codes[i] {
				codes[i], codes[j] = codes[j], codes[i]
			}
		}
	}
	out := make([]CantonCapability, 0, len(codes))
	for _, c := range codes {
		out = append(out, Cantons[c])
	}
	return out
}

// firstString and firstInt read the first non-empty value from an attribute
// map across a list of candidate keys — cantonal layers vary in the exact
// attribute names they use.
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			if n, ok := v.(float64); ok {
				return strconv.FormatFloat(n, 'f', -1, 64)
			}
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int(t)
			case int:
				return t
			case json.Number:
				n, err := t.Int64()
				if err == nil {
					return int(n)
				}
			case string:
				if n, err := strconv.Atoi(t); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func floatAttr(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return t, true
			case json.Number:
				if n, err := t.Float64(); err == nil {
					return n, true
				}
			case string:
				if n, err := strconv.ParseFloat(t, 64); err == nil {
					return n, true
				}
			}
		}
	}
	return 0, false
}

// GrundbuchHit is a candidate from the address/parcel search.
type GrundbuchHit struct {
	Label        string  `json:"label"`
	Detail       string  `json:"detail,omitempty"`
	Layer        string  `json:"layer,omitempty"`
	Origin       string  `json:"origin,omitempty"`
	LV95E        float64 `json:"lv95_e"`
	LV95N        float64 `json:"lv95_n"`
	Municipality string  `json:"municipality,omitempty"`
	BFS          int     `json:"bfs,omitempty"`
	Canton       string  `json:"canton,omitempty"`
}

// ErrNoAdapter is returned by owner-lookup paths when a canton has no live
// adapter wired up. Callers should fall through to the capability how-to
// block.
var ErrNoAdapter = errors.New("no live owner adapter for this canton")

// stripHTMLTags removes the minimal tag markup SearchServer embeds in labels
// (<b>…</b>). Mirrors cmd/geo.go's stripGeoHTML but kept local so this file
// compiles without package-level cross-references.
func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
