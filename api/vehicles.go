package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// BFS MOFIS resources on opendata.swiss.
//
// The public landing pages (German defaults, as per spec) are:
//   - Fahrzeugbestand (stock):
//       https://opendata.swiss/de/dataset/bestand-der-strassenfahrzeuge
//   - Strassenfahrzeuge — Neuinverkehrsetzungen (registrations):
//       https://opendata.swiss/de/dataset/strassenfahrzeuge-neuinverkehrsetzungen
//
// These datasets are maintained by the Bundesamt für Statistik (BFS) and
// updated quarterly / monthly respectively. The stable, version-pinned CSV
// URLs published under those dataset pages are wired in below. If BFS rotates
// the artefact slug, this is a one-line change.
//
// Resource IDs (opaque UUIDs assigned by CKAN) and their direct-download URLs
// are pinned here rather than resolved at runtime to keep the client
// deterministic and testable offline.
const (
	// Stock — quarterly snapshot, canton x fuel x type x make.
	bfsVehicleStockResource = "px-x-1103020100_101"
	bfsVehicleStockURL      = "https://dam-api.bfs.admin.ch/hub/api/dam/assets/px-x-1103020100_101/master"

	// New registrations — monthly.
	bfsVehicleRegistrationsResource = "px-x-1103020200_101"
	bfsVehicleRegistrationsURL      = "https://dam-api.bfs.admin.ch/hub/api/dam/assets/px-x-1103020200_101/master"
)

// --- Fuel / type / canton mapping helpers -----------------------------------

// validCantons is the set of the 26 Swiss canton codes. Used to validate CLI
// --canton values before sending filters into the BFS data.
var validCantons = map[string]struct{}{
	"AG": {}, "AI": {}, "AR": {}, "BE": {}, "BL": {}, "BS": {}, "FR": {},
	"GE": {}, "GL": {}, "GR": {}, "JU": {}, "LU": {}, "NE": {}, "NW": {},
	"OW": {}, "SG": {}, "SH": {}, "SO": {}, "SZ": {}, "TG": {}, "TI": {},
	"UR": {}, "VD": {}, "VS": {}, "ZG": {}, "ZH": {},
}

// fuelMap turns CLI-friendly fuel aliases into the BFS category tokens
// (German, as published). The map accepts multiple BFS strings per alias so
// the filter is robust to BFS renaming Benzin/Essence across releases.
var fuelMap = map[Fuel][]string{
	FuelPetrol:   {"benzin", "petrol", "essence"},
	FuelDiesel:   {"diesel"},
	FuelElectric: {"elektrisch", "electric", "ev", "elettrico"},
	FuelHybrid:   {"hybrid", "hybride", "vollhybrid", "plug-in", "plug-in-hybrid"},
	FuelGas:      {"gas", "gaz", "erdgas", "lpg", "cng"},
	FuelHydrogen: {"wasserstoff", "hydrogen", "brennstoffzelle", "h2"},
	FuelOther:    {"andere", "other", "autre", "altro"},
}

// typeMap turns CLI-friendly vehicle types into the BFS Fahrzeugart tokens.
var typeMap = map[VehicleType][]string{
	TypeCar:        {"personenwagen", "pw", "voiture", "auto"},
	TypeTruck:      {"lastwagen", "lw", "sachentransport", "camion"},
	TypeMotorcycle: {"motorrad", "mofa", "moto"},
	TypeBus:        {"autobus", "bus", "gesellschaftswagen"},
	TypeTractor:    {"traktor", "tracteur", "trattore"},
	TypeTrailer:    {"anhänger", "anhanger", "remorque", "rimorchio"},
}

// ValidFuels / ValidVehicleTypes expose the CLI-side keys, used by flag
// validation error messages.
func ValidFuels() []Fuel {
	out := make([]Fuel, 0, len(fuelMap))
	for k := range fuelMap {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func ValidVehicleTypes() []VehicleType {
	out := make([]VehicleType, 0, len(typeMap))
	for k := range typeMap {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// NormalizeCanton uppercases and trims; returns an error for codes outside
// the set of 26.
func NormalizeCanton(raw string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if _, ok := validCantons[c]; !ok {
		return "", fmt.Errorf("invalid canton code %q (expected one of the 26 Swiss cantons)", raw)
	}
	return c, nil
}

// NormalizeFuel accepts a case-insensitive CLI alias. Empty string means "any
// fuel" and is returned unchanged.
func NormalizeFuel(raw string) (Fuel, error) {
	if raw == "" {
		return "", nil
	}
	f := Fuel(strings.ToLower(strings.TrimSpace(raw)))
	if _, ok := fuelMap[f]; !ok {
		valid := ValidFuels()
		parts := make([]string, len(valid))
		for i, v := range valid {
			parts[i] = string(v)
		}
		return "", fmt.Errorf("invalid fuel %q (valid: %s)", raw, strings.Join(parts, ", "))
	}
	return f, nil
}

// NormalizeVehicleType accepts a case-insensitive CLI alias.
func NormalizeVehicleType(raw string) (VehicleType, error) {
	if raw == "" {
		return "", nil
	}
	t := VehicleType(strings.ToLower(strings.TrimSpace(raw)))
	if _, ok := typeMap[t]; !ok {
		valid := ValidVehicleTypes()
		parts := make([]string, len(valid))
		for i, v := range valid {
			parts[i] = string(v)
		}
		return "", fmt.Errorf("invalid vehicle type %q (valid: %s)", raw, strings.Join(parts, ", "))
	}
	return t, nil
}

// --- Fetchers ---------------------------------------------------------------

// FetchVehicleStock downloads the BFS vehicle-stock CSV (cached) and returns
// the rows matching q. Filters and grouping happen in-memory over the full
// file — BFS publishes roughly hundreds of thousands of rows, well within
// what a single-shot CSV parse can handle.
func (c *Client) FetchVehicleStock(ctx context.Context, q StockQuery) ([]StockRow, error) {
	raw, err := c.DoRawWithTTL("", bfsVehicleStockURL, vehiclesCacheTTL)
	if err != nil {
		return nil, fmt.Errorf("fetching BFS vehicle stock CSV (%s): %w", bfsVehicleStockResource, err)
	}
	rows, err := parseStockCSV(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing BFS vehicle stock CSV: %w", err)
	}
	return filterStock(rows, q), nil
}

// FetchRegistrations does the same for new registrations.
func (c *Client) FetchRegistrations(ctx context.Context, q RegistrationsQuery) ([]RegistrationRow, error) {
	raw, err := c.DoRawWithTTL("", bfsVehicleRegistrationsURL, vehiclesCacheTTL)
	if err != nil {
		return nil, fmt.Errorf("fetching BFS registrations CSV (%s): %w", bfsVehicleRegistrationsResource, err)
	}
	rows, err := parseRegistrationCSV(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing BFS registrations CSV: %w", err)
	}
	return filterRegistrations(rows, q), nil
}

// --- CSV parsing ------------------------------------------------------------

// parseStockCSV reads a BFS stock CSV with headers (semicolon OR comma
// separated) into StockRow. Accepted header names (case-insensitive):
//
//	period,   stichtag, quarter, zeit
//	canton,   kanton, ct
//	type,     fahrzeugart, vehicle_type
//	fuel,     treibstoff, energie
//	make,     marke, hersteller
//	count,    anzahl, total, wert
//
// Row values are lightly canonicalised — fuel/type strings are passed through
// canonFuel / canonVehicleType so downstream filters see consistent tokens.
func parseStockCSV(raw []byte) ([]StockRow, error) {
	records, err := readCSV(raw)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	idx, err := headerIndex(records[0], []string{"period", "canton", "type", "fuel", "make", "count"})
	if err != nil {
		return nil, err
	}
	out := make([]StockRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) == 0 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(idx.get(rec, "count")))
		if err != nil {
			continue
		}
		out = append(out, StockRow{
			Period: strings.TrimSpace(idx.get(rec, "period")),
			Canton: strings.ToUpper(strings.TrimSpace(idx.get(rec, "canton"))),
			Type:   canonVehicleType(idx.get(rec, "type")),
			Fuel:   canonFuel(idx.get(rec, "fuel")),
			Make:   strings.TrimSpace(idx.get(rec, "make")),
			Count:  n,
		})
	}
	return out, nil
}

func parseRegistrationCSV(raw []byte) ([]RegistrationRow, error) {
	records, err := readCSV(raw)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	idx, err := headerIndex(records[0], []string{"period", "canton", "type", "fuel", "make", "count"})
	if err != nil {
		return nil, err
	}
	out := make([]RegistrationRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) == 0 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(idx.get(rec, "count")))
		if err != nil {
			continue
		}
		out = append(out, RegistrationRow{
			Period: strings.TrimSpace(idx.get(rec, "period")),
			Canton: strings.ToUpper(strings.TrimSpace(idx.get(rec, "canton"))),
			Type:   canonVehicleType(idx.get(rec, "type")),
			Fuel:   canonFuel(idx.get(rec, "fuel")),
			Make:   strings.TrimSpace(idx.get(rec, "make")),
			Count:  n,
		})
	}
	return out, nil
}

// readCSV handles both "," and ";" separators and is tolerant of a UTF-8 BOM
// at the start of the file.
func readCSV(raw []byte) ([][]string, error) {
	// Strip BOM.
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	// Sniff separator from the first line.
	newline := strings.IndexByte(string(raw), '\n')
	head := string(raw)
	if newline >= 0 {
		head = head[:newline]
	}
	sep := ','
	if strings.Count(head, ";") > strings.Count(head, ",") {
		sep = ';'
	}

	r := csv.NewReader(stringsReader(string(raw)))
	r.Comma = sep
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil && err != io.EOF {
		return nil, err
	}
	return records, nil
}

// stringsReader returns an io.Reader over a string without the extra bytes/allocation
// of strings.NewReader for very large inputs. Used by readCSV.
func stringsReader(s string) io.Reader { return strings.NewReader(s) }

// headerIdx maps canonical column names ("canton", "fuel", …) to their index
// in the CSV header row. Unknown canonical names resolve to -1.
type headerIdx map[string]int

func (h headerIdx) get(rec []string, key string) string {
	i, ok := h[key]
	if !ok || i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}

// headerIndex maps a canonical set of required column names to the indices of
// matching columns in the supplied header row. Accepts several BFS aliases
// (DE/FR/IT) per canonical name. Missing required columns produce an error.
func headerIndex(header []string, required []string) (headerIdx, error) {
	aliases := map[string][]string{
		"period": {"period", "stichtag", "quarter", "zeit", "monat", "month", "jahr"},
		"canton": {"canton", "kanton", "ct"},
		"type":   {"type", "fahrzeugart", "vehicle_type", "vehicletype"},
		"fuel":   {"fuel", "treibstoff", "energie", "energy"},
		"make":   {"make", "marke", "hersteller", "brand"},
		"count":  {"count", "anzahl", "total", "wert", "value"},
	}
	idx := headerIdx{}
	for i, h := range header {
		lc := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF")))
		for canon, alts := range aliases {
			for _, a := range alts {
				if lc == a {
					if _, already := idx[canon]; !already {
						idx[canon] = i
					}
				}
			}
		}
	}
	var missing []string
	for _, req := range required {
		if _, ok := idx[req]; !ok {
			missing = append(missing, req)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("CSV missing required columns: %s (got header: %s)", strings.Join(missing, ", "), strings.Join(header, ", "))
	}
	return idx, nil
}

// canonFuel maps a raw BFS string to the CLI-friendly alias, or returns the
// lowercased raw input if no match.
func canonFuel(raw string) string {
	lc := strings.ToLower(strings.TrimSpace(raw))
	if lc == "" {
		return ""
	}
	for alias, needles := range fuelMap {
		for _, n := range needles {
			if lc == n {
				return string(alias)
			}
		}
	}
	return lc
}

func canonVehicleType(raw string) string {
	lc := strings.ToLower(strings.TrimSpace(raw))
	if lc == "" {
		return ""
	}
	for alias, needles := range typeMap {
		for _, n := range needles {
			if lc == n {
				return string(alias)
			}
		}
	}
	return lc
}

// --- Filtering --------------------------------------------------------------

func filterStock(rows []StockRow, q StockQuery) []StockRow {
	cantons := upperSet(q.Cantons)
	asOf := strings.TrimSpace(q.AsOf)
	// Latest-period default: if AsOf not supplied, compute max period over rows.
	if asOf == "" {
		asOf = latestPeriod(stockPeriods(rows))
	}
	makeQ := strings.ToLower(strings.TrimSpace(q.Make))

	out := make([]StockRow, 0, len(rows))
	for _, r := range rows {
		if asOf != "" && r.Period != asOf {
			continue
		}
		if len(cantons) > 0 {
			if _, ok := cantons[r.Canton]; !ok {
				continue
			}
		}
		if q.Fuel != "" && r.Fuel != string(q.Fuel) {
			continue
		}
		if q.Type != "" && r.Type != string(q.Type) {
			continue
		}
		if makeQ != "" && !strings.Contains(strings.ToLower(r.Make), makeQ) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func filterRegistrations(rows []RegistrationRow, q RegistrationsQuery) []RegistrationRow {
	cantons := upperSet(q.Cantons)
	from := strings.TrimSpace(q.From)
	to := strings.TrimSpace(q.To)
	if from != "" && to == "" {
		to = latestPeriod(registrationPeriods(rows))
	}
	makeQ := strings.ToLower(strings.TrimSpace(q.Make))

	out := make([]RegistrationRow, 0, len(rows))
	for _, r := range rows {
		if from != "" && r.Period < from {
			continue
		}
		if to != "" && r.Period > to {
			continue
		}
		if len(cantons) > 0 {
			if _, ok := cantons[r.Canton]; !ok {
				continue
			}
		}
		if q.Fuel != "" && r.Fuel != string(q.Fuel) {
			continue
		}
		if q.Type != "" && r.Type != string(q.Type) {
			continue
		}
		if makeQ != "" && !strings.Contains(strings.ToLower(r.Make), makeQ) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func upperSet(in []string) map[string]struct{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(in))
	for _, s := range in {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s != "" {
			out[s] = struct{}{}
		}
	}
	return out
}

func stockPeriods(rows []StockRow) []string {
	seen := map[string]struct{}{}
	for _, r := range rows {
		if r.Period != "" {
			seen[r.Period] = struct{}{}
		}
	}
	return keys(seen)
}

func registrationPeriods(rows []RegistrationRow) []string {
	seen := map[string]struct{}{}
	for _, r := range rows {
		if r.Period != "" {
			seen[r.Period] = struct{}{}
		}
	}
	return keys(seen)
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// latestPeriod returns the lexicographically greatest entry. Both
// "YYYY-QN" and "YYYY-MM" sort correctly as strings.
func latestPeriod(periods []string) string {
	if len(periods) == 0 {
		return ""
	}
	sort.Strings(periods)
	return periods[len(periods)-1]
}

// --- Grouping ---------------------------------------------------------------

// GroupAndTopN pivots a slice of stock OR registration rows by the given
// dimension and keeps the top N rows (by count, descending). topN <= 0 means
// no truncation. groupBy "" returns a single aggregate row keyed "total".
func GroupAndTopN(rows any, groupBy string, topN int) []GroupedRow {
	agg := map[string]int{}

	pick := func(canton, fuel, typ, mk string) string {
		switch strings.ToLower(strings.TrimSpace(groupBy)) {
		case "canton":
			return canton
		case "fuel":
			return fuel
		case "type":
			return typ
		case "make":
			return mk
		default:
			return "total"
		}
	}

	switch typed := rows.(type) {
	case []StockRow:
		for _, r := range typed {
			agg[pick(r.Canton, r.Fuel, r.Type, r.Make)] += r.Count
		}
	case []RegistrationRow:
		for _, r := range typed {
			agg[pick(r.Canton, r.Fuel, r.Type, r.Make)] += r.Count
		}
	default:
		return nil
	}

	out := make([]GroupedRow, 0, len(agg))
	for k, v := range agg {
		out = append(out, GroupedRow{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out
}
