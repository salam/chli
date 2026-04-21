package api

// Fuel is a CLI-friendly fuel-category alias. Internally mapped to the BFS
// column codes used in the MOFIS CSV (e.g. "Benzin", "Diesel", "Elektrisch").
type Fuel string

const (
	FuelPetrol   Fuel = "petrol"
	FuelDiesel   Fuel = "diesel"
	FuelElectric Fuel = "electric"
	FuelHybrid   Fuel = "hybrid"
	FuelGas      Fuel = "gas"
	FuelHydrogen Fuel = "hydrogen"
	FuelOther    Fuel = "other"
)

// VehicleType is a CLI-friendly vehicle category alias. Internally mapped to
// the BFS Fahrzeugart codes (PW, LW, Mofa, Motorrad, Autobus, Traktor, …).
type VehicleType string

const (
	TypeCar        VehicleType = "car"
	TypeTruck      VehicleType = "truck"
	TypeMotorcycle VehicleType = "motorcycle"
	TypeBus        VehicleType = "bus"
	TypeTractor    VehicleType = "tractor"
	TypeTrailer    VehicleType = "trailer"
)

// StockQuery narrows a stock snapshot by canton / fuel / type / make and the
// reporting period (quarter). All fields are optional — the zero value returns
// the full national snapshot for the latest available quarter.
type StockQuery struct {
	Cantons  []string    // e.g. ["ZH", "BE"]; empty = all cantons
	Fuel     Fuel        // "" = any fuel
	Type     VehicleType // "" = any vehicle type
	Make     string      // substring match, case-insensitive; "" = any make
	AsOf     string      // "YYYY-QN"; "" = latest snapshot
	GroupBy  string      // "canton"|"fuel"|"make"|"type"|"" (no grouping)
	TopN     int         // <=0 means no truncation; positive keeps the top N rows
}

// StockRow is a single post-filter row in the stock dataset. Count is the
// number of vehicles recorded for that cell in the requested snapshot.
type StockRow struct {
	Period  string `json:"period"`  // e.g. "2026-Q1"
	Canton  string `json:"canton"`  // e.g. "ZH"
	Type    string `json:"type"`    // CLI-friendly type (car/truck/…)
	Fuel    string `json:"fuel"`    // CLI-friendly fuel
	Make    string `json:"make"`    // raw make string from BFS
	Count   int    `json:"count"`
}

// RegistrationsQuery narrows new-registration rows. Same filters as StockQuery
// but the time window is a [From, To] inclusive pair of months.
type RegistrationsQuery struct {
	Cantons []string
	Fuel    Fuel
	Type    VehicleType
	Make    string
	From    string // "YYYY-MM"; "" = earliest available
	To      string // "YYYY-MM"; "" = latest available
	GroupBy string
	TopN    int
}

// RegistrationRow is a single post-filter new-registration record.
type RegistrationRow struct {
	Period string `json:"period"` // "YYYY-MM"
	Canton string `json:"canton"`
	Type   string `json:"type"`
	Fuel   string `json:"fuel"`
	Make   string `json:"make"`
	Count  int    `json:"count"`
}

// GroupedRow is the aggregated output produced by GroupAndTopN. Key is the
// value of the grouping dimension (e.g. "ZH" for group-by canton). Count is
// the sum across all rows contributing to that key.
type GroupedRow struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}
