package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlateQuota_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	q, err := LoadPlateQuota(dir)
	if err != nil {
		t.Fatalf("LoadPlateQuota on empty dir: %v", err)
	}
	if got := q.Remaining(time.Now().UTC()); got != PlateQuotaMax {
		t.Errorf("Remaining on empty quota = %d, want %d", got, PlateQuotaMax)
	}
}

func TestPlateQuota_RecordAndReload(t *testing.T) {
	dir := t.TempDir()
	q := &PlateQuota{}
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	if err := q.Record(dir, QuotaEntry{Timestamp: now, Canton: "FR", Plate: "FR999999"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	reloaded, err := LoadPlateQuota(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got, want := reloaded.Remaining(now), PlateQuotaMax-1; got != want {
		t.Errorf("Remaining after 1 record = %d, want %d", got, want)
	}
	active := reloaded.Active(now)
	if len(active) != 1 || active[0].Canton != "FR" {
		t.Errorf("Active after 1 record = %v, want [{FR ...}]", active)
	}
}

func TestPlateQuota_CapAndNextAvailable(t *testing.T) {
	dir := t.TempDir()
	q := &PlateQuota{}
	start := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	for i := 0; i < PlateQuotaMax; i++ {
		ts := start.Add(time.Duration(i) * time.Minute)
		if err := q.Record(dir, QuotaEntry{Timestamp: ts, Canton: "FR", Plate: "FR000000"}); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}

	now := start.Add(5 * time.Minute)
	if got := q.Remaining(now); got != 0 {
		t.Errorf("Remaining at cap = %d, want 0", got)
	}
	wantNext := start.Add(PlateQuotaWindow)
	if got := q.NextAvailable(now); !got.Equal(wantNext) {
		t.Errorf("NextAvailable = %v, want %v", got, wantNext)
	}
}

func TestPlateQuota_Pruning(t *testing.T) {
	dir := t.TempDir()
	q := &PlateQuota{}
	oldTs := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	newTs := oldTs.Add(PlateQuotaWindow + time.Hour) // 25 hours later
	if err := q.Record(dir, QuotaEntry{Timestamp: oldTs, Canton: "FR", Plate: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := q.Record(dir, QuotaEntry{Timestamp: newTs, Canton: "VS", Plate: "B"}); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadPlateQuota(dir)
	if err != nil {
		t.Fatal(err)
	}
	active := reloaded.Active(newTs)
	if len(active) != 1 || active[0].Canton != "VS" {
		t.Errorf("Active after pruning = %v, want [{VS ...}]", active)
	}
}

func TestPlateQuota_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plate-quota.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlateQuota(dir); err == nil {
		t.Error("LoadPlateQuota on corrupt file = nil error, want decode error")
	}
}
