package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Language string `json:"language"`
	CacheDir string `json:"cache_dir"`
}

func DefaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/chli-cache"
	}
	return filepath.Join(home, ".cache", "chli")
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "chli"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultConfig(), nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), nil
	}
	if cfg.Language == "" {
		cfg.Language = "de"
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = DefaultCacheDir()
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")
	return os.WriteFile(path, data, 0644)
}

func defaultConfig() *Config {
	return &Config{
		Language: "de",
		CacheDir: DefaultCacheDir(),
	}
}
