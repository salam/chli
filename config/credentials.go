package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds per-service authentication secrets. Stored separately
// from the main config so the config file can stay shareable while
// credentials are kept in a 0600 file.
type Credentials struct {
	Services map[string]ServiceCreds `json:"services"`
}

type ServiceCreds struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

func credentialsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// LoadCredentials reads credentials.json. Missing file returns an empty
// struct and no error.
func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return &Credentials{Services: map[string]ServiceCreds{}}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{Services: map[string]ServiceCreds{}}, nil
		}
		return nil, fmt.Errorf("reading credentials: %w", err)
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}
	if c.Services == nil {
		c.Services = map[string]ServiceCreds{}
	}
	return &c, nil
}

// Save writes credentials.json with 0600 permissions.
func (c *Credentials) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "credentials.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// Write to temp + rename so we never leave a partial file around.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// Get returns credentials for a service, or an empty ServiceCreds if absent.
func (c *Credentials) Get(service string) ServiceCreds {
	if c == nil || c.Services == nil {
		return ServiceCreds{}
	}
	return c.Services[service]
}

// Set stores credentials for a service.
func (c *Credentials) Set(service string, creds ServiceCreds) {
	if c.Services == nil {
		c.Services = map[string]ServiceCreds{}
	}
	c.Services[service] = creds
}

// Delete removes a service from the store.
func (c *Credentials) Delete(service string) {
	delete(c.Services, service)
}

// Path returns the on-disk location of the credentials file (for display).
func CredentialsPath() string {
	p, _ := credentialsPath()
	return p
}
