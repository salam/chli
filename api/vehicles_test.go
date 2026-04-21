package api

import (
	"context"
	"os"
	"sort"
	"testing"
)

func loadStockFixture(t *testing.T) []StockRow {
	t.Helper()
	raw, err := os.ReadFile("testdata/vehicles-stock.csv")
	if err != nil {
		t.Fatalf("read stock fixture: %v", err)
	}
	rows, err := parseStockCSV(raw)
	if err != nil {
		t.Fatalf("parse stock fixture: %v", err)
	}
	return rows
}

func loadRegistrationFixture(t *testing.T) []RegistrationRow {
	t.Helper()
	raw, err := os.ReadFile("testdata/vehicles-registrations.csv")
	if err != nil {
		t.Fatalf("read registrations fixture: %v", err)
	}
	rows, err := parseRegistrationCSV(raw)
	if err != nil {
		t.Fatalf("parse registrations fixture: %v", err)
	}
	return rows
}

func TestParseStockCSV(t *testing.T) {
	rows := loadStockFixture(t)
	if len(rows) < 25 {
		t.Fatalf("expected at least 25 rows from fixture, got %d", len(rows))
	}

	// Header aliases should have been resolved: fuel/type canonicalised.
	var sawElectric, sawCar, sawMotorcycle bool
	for _, r := range rows {
		switch r.Fuel {
		case string(FuelElectric):
			sawElectric = true
		}
		switch r.Type {
		case string(TypeCar):
			sawCar = true
		case string(TypeMotorcycle):
			sawMotorcycle = true
		}
		if r.Count <= 0 {
			t.Errorf("unexpected non-positive count: %+v", r)
		}
	}
	if !sawElectric {
		t.Errorf("expected at least one electric row in fixture")
	}
	if !sawCar || !sawMotorcycle {
		t.Errorf("expected car+motorcycle rows in fixture (car=%v, moto=%v)", sawCar, sawMotorcycle)
	}
}

func TestFilterStockByCanton(t *testing.T) {
	rows := loadStockFixture(t)
	got := filterStock(rows, StockQuery{
		Cantons: []string{"zh", "BE"}, // mixed case — NormalizeCanton pipeline
		AsOf:    "2026-Q1",
	})
	if len(got) == 0 {
		t.Fatal("expected canton filter to return rows")
	}
	for _, r := range got {
		if r.Canton != "ZH" && r.Canton != "BE" {
			t.Errorf("unexpected canton %q", r.Canton)
		}
		if r.Period != "2026-Q1" {
			t.Errorf("period %q not pinned to 2026-Q1", r.Period)
		}
	}
}

func TestFilterStockByFuelAndMake(t *testing.T) {
	rows := loadStockFixture(t)
	got := filterStock(rows, StockQuery{
		Fuel: FuelElectric,
		Make: "tesla",
		AsOf: "2026-Q1",
	})
	if len(got) == 0 {
		t.Fatal("expected tesla/electric rows")
	}
	for _, r := range got {
		if r.Fuel != string(FuelElectric) {
			t.Errorf("unexpected fuel %q", r.Fuel)
		}
		if r.Make != "Tesla" {
			t.Errorf("unexpected make %q", r.Make)
		}
	}
}

func TestFilterStockDefaultsToLatestPeriod(t *testing.T) {
	rows := loadStockFixture(t)
	// AsOf empty -> latest period in fixture (2026-Q1 > 2025-Q4).
	got := filterStock(rows, StockQuery{})
	if len(got) == 0 {
		t.Fatal("expected rows for latest period")
	}
	for _, r := range got {
		if r.Period != "2026-Q1" {
			t.Errorf("expected latest period 2026-Q1, got %q", r.Period)
		}
	}
}

func TestGroupAndTopNByCanton(t *testing.T) {
	rows := loadStockFixture(t)
	latest := filterStock(rows, StockQuery{AsOf: "2026-Q1"})

	all := GroupAndTopN(latest, "canton", 0)
	if len(all) != 8 {
		t.Fatalf("expected 8 distinct cantons in Q1 fixture, got %d (%+v)", len(all), all)
	}
	// Ensure sorted descending by count.
	for i := 1; i < len(all); i++ {
		if all[i-1].Count < all[i].Count {
			t.Errorf("groups not sorted desc: %+v", all)
		}
	}

	top3 := GroupAndTopN(latest, "canton", 3)
	if len(top3) != 3 {
		t.Fatalf("top 3 truncation broken: got %d", len(top3))
	}
	if top3[0].Key != "ZH" {
		t.Errorf("expected ZH to lead, got %q", top3[0].Key)
	}
}

func TestGroupAndTopNByFuel(t *testing.T) {
	rows := loadStockFixture(t)
	latest := filterStock(rows, StockQuery{AsOf: "2026-Q1"})
	grouped := GroupAndTopN(latest, "fuel", 0)

	keys := make([]string, 0, len(grouped))
	for _, g := range grouped {
		keys = append(keys, g.Key)
	}
	sort.Strings(keys)
	want := []string{string(FuelDiesel), string(FuelElectric), string(FuelHybrid), string(FuelPetrol)}
	sort.Strings(want)
	if !equalStrings(keys, want) {
		t.Errorf("fuel keys = %v, want %v", keys, want)
	}
}

func TestGroupAndTopNRegistrationsPeriodFilter(t *testing.T) {
	rows := loadRegistrationFixture(t)
	got := filterRegistrations(rows, RegistrationsQuery{
		From: "2026-01",
		To:   "2026-02",
	})
	if len(got) == 0 {
		t.Fatal("no rows in 2026-01..2026-02 range")
	}
	for _, r := range got {
		if r.Period < "2026-01" || r.Period > "2026-02" {
			t.Errorf("row %+v escaped the period window", r)
		}
	}

	// --from only => defaults --to to latest in set (2026-03).
	rolling := filterRegistrations(rows, RegistrationsQuery{From: "2026-02"})
	for _, r := range rolling {
		if r.Period < "2026-02" {
			t.Errorf("rolling window leaked pre-2026-02 row: %+v", r)
		}
	}
}

func TestNormalizeFuelAndType(t *testing.T) {
	if _, err := NormalizeFuel("DIESEL"); err != nil {
		t.Errorf("DIESEL should normalize: %v", err)
	}
	if _, err := NormalizeFuel("solar"); err == nil {
		t.Error("solar should error")
	}
	if _, err := NormalizeVehicleType("Car"); err != nil {
		t.Errorf("Car should normalize: %v", err)
	}
	if _, err := NormalizeVehicleType("scooter"); err == nil {
		t.Error("scooter should error")
	}
}

func TestNormalizeCanton(t *testing.T) {
	got, err := NormalizeCanton("zh ")
	if err != nil || got != "ZH" {
		t.Errorf("NormalizeCanton(zh )=%q,%v; want ZH,nil", got, err)
	}
	if _, err := NormalizeCanton("XX"); err == nil {
		t.Error("XX should be rejected")
	}
}

// Env-gated integration test: actually hit opendata.swiss. Runs only when
// CHLI_INTEGRATION_TESTS=1 so CI / dev laptops don't pay for network flake
// on every `go test`.
func TestIntegration_FetchVehicleStock(t *testing.T) {
	if os.Getenv("CHLI_INTEGRATION_TESTS") != "1" {
		t.Skip("set CHLI_INTEGRATION_TESTS=1 to run integration tests")
	}
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	rows, err := c.FetchVehicleStock(context.Background(), StockQuery{Cantons: []string{"ZH"}})
	if err != nil {
		t.Fatalf("FetchVehicleStock: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("integration: expected at least one row for ZH")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
