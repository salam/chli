package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	// Load() returns defaults when no config file exists.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Language != "de" {
		t.Errorf("Language = %q, want %q", cfg.Language, "de")
	}
	if cfg.CacheDir == "" {
		t.Error("CacheDir is empty, want non-empty default")
	}
}

func TestDefaultCacheDir(t *testing.T) {
	dir := DefaultCacheDir()
	if dir == "" {
		t.Error("DefaultCacheDir() returned empty string")
	}
	// Should contain "chli" somewhere in the path.
	if len(dir) < 4 {
		t.Errorf("DefaultCacheDir() = %q, seems too short", dir)
	}
}
