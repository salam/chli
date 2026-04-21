package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// PlateQuotaMax is the cap on `chli plate <...> --query` invocations in a
// rolling 24-hour window. This is a single global counter, not per-canton —
// if you've used 3 browser launches today (across any cantons), you're done
// until the oldest one ages out.
//
// The cap exists to (a) respect cantonal daily limits (several cantons cap
// at 3/day themselves; we keep chli under the strictest) and (b) gently
// discourage casual lookups. It is enforced client-side only; the canton's
// own captcha and cap remain the real gate.
const PlateQuotaMax = 3

// PlateQuotaWindow is the rolling window within which PlateQuotaMax applies.
const PlateQuotaWindow = 24 * time.Hour

// QuotaEntry records one --query invocation.
type QuotaEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Canton    string    `json:"canton"`
	Plate     string    `json:"plate"`
	URL       string    `json:"url,omitempty"`
}

// PlateQuota is the on-disk shape of the quota file.
type PlateQuota struct {
	Entries []QuotaEntry `json:"entries"`
}

// PlateQuotaPath returns the path to the quota file under the user's cache
// directory. Missing directories are not created here; Record() creates
// them on first write.
func PlateQuotaPath(cacheDir string) string {
	return filepath.Join(cacheDir, "plate-quota.json")
}

// LoadPlateQuota reads the quota file. A missing file returns an empty
// quota and no error — first-run case.
func LoadPlateQuota(cacheDir string) (*PlateQuota, error) {
	path := PlateQuotaPath(cacheDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PlateQuota{}, nil
		}
		return nil, fmt.Errorf("reading plate quota %s: %w", path, err)
	}
	var q PlateQuota
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("parsing plate quota %s: %w", path, err)
	}
	return &q, nil
}

// Active returns entries whose timestamp falls within the last window. Older
// entries are pruned (caller decides whether to persist).
func (q *PlateQuota) Active(now time.Time) []QuotaEntry {
	cutoff := now.Add(-PlateQuotaWindow)
	active := make([]QuotaEntry, 0, len(q.Entries))
	for _, e := range q.Entries {
		if e.Timestamp.After(cutoff) {
			active = append(active, e)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].Timestamp.Before(active[j].Timestamp)
	})
	return active
}

// Remaining reports how many --query invocations are still allowed right now.
func (q *PlateQuota) Remaining(now time.Time) int {
	left := PlateQuotaMax - len(q.Active(now))
	if left < 0 {
		return 0
	}
	return left
}

// NextAvailable returns the timestamp at which one more slot will open up.
// Valid only when Remaining() == 0; otherwise returns the zero Time.
func (q *PlateQuota) NextAvailable(now time.Time) time.Time {
	active := q.Active(now)
	if len(active) < PlateQuotaMax {
		return time.Time{}
	}
	return active[0].Timestamp.Add(PlateQuotaWindow)
}

// Record appends one entry, prunes the window, and writes the file.
// Callers should check Remaining() first; Record does not enforce the cap.
func (q *PlateQuota) Record(cacheDir string, entry QuotaEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	q.Entries = append(q.Active(entry.Timestamp), entry)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir %s: %w", cacheDir, err)
	}
	path := PlateQuotaPath(cacheDir)
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding plate quota: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing plate quota %s: %w", path, err)
	}
	return nil
}
