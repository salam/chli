package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cross-source cache TTLs that live outside their own source file. Most
// sources declare their TTL in the source file they apply to; these are
// shared with subcommands whose data loaders or endpoint probes would
// otherwise have no natural home.
const (
	plateCacheTTL    = 24 * time.Hour
	vehiclesCacheTTL = 24 * time.Hour
)

type Cache struct {
	Dir string
}

type cacheEntry struct {
	Data      []byte `json:"data"`
	Timestamp int64  `json:"timestamp"`
	TTL       int64  `json:"ttl"`
}

func NewCache(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}
	return &Cache{Dir: dir}, nil
}

func (c *Cache) key(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)
}

func (c *Cache) Get(rawKey string) ([]byte, bool) {
	path := filepath.Join(c.Dir, c.key(rawKey)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	age := time.Now().Unix() - entry.Timestamp
	if age > entry.TTL {
		os.Remove(path)
		return nil, false
	}
	return entry.Data, true
}

func (c *Cache) Set(rawKey string, data []byte, ttl time.Duration) {
	entry := cacheEntry{
		Data:      data,
		Timestamp: time.Now().Unix(),
		TTL:       int64(ttl.Seconds()),
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := filepath.Join(c.Dir, c.key(rawKey)+".json")
	os.WriteFile(path, encoded, 0644)
}

func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		os.Remove(filepath.Join(c.Dir, e.Name()))
	}
	return nil
}
