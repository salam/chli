package api

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed plate_cantons.json
var plateCantonsRaw []byte

var (
	plateCantonsOnce sync.Once
	plateCantonsData map[string]CantonEntry
	plateCantonsErr  error
)

// LoadCantons parses and validates the embedded plate_cantons.json. It is
// safe for concurrent use and guaranteed to run validation at most once per
// process. A malformed file is a build-time bug and produces a wrapped error
// the caller should treat as fatal.
func LoadCantons() (map[string]CantonEntry, error) {
	plateCantonsOnce.Do(func() {
		plateCantonsData, plateCantonsErr = decodeCantons(plateCantonsRaw)
	})
	if plateCantonsErr != nil {
		return nil, plateCantonsErr
	}
	return plateCantonsData, nil
}

// decodeCantons is the pure decoder + validator; exported for testing via
// loadCantonsFromBytes.
func decodeCantons(raw []byte) (map[string]CantonEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	var file PlateCantonsFile
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode plate_cantons.json: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("decode plate_cantons.json: trailing data after top-level object")
	}

	if file.SchemaVersion != 1 {
		return nil, fmt.Errorf("plate_cantons.json: schema_version %d not supported (want 1)", file.SchemaVersion)
	}
	if file.Cantons == nil {
		return nil, fmt.Errorf("plate_cantons.json: cantons object missing")
	}
	if len(file.Cantons) != len(ValidCantons) {
		return nil, fmt.Errorf("plate_cantons.json: got %d entries, want %d", len(file.Cantons), len(ValidCantons))
	}

	// Ensure exact set match and copy the code into the entry.
	out := make(map[string]CantonEntry, len(file.Cantons))
	for code, entry := range file.Cantons {
		if _, ok := ValidCantons[code]; !ok {
			return nil, fmt.Errorf("plate_cantons.json: unknown canton code %q", code)
		}
		entry.Code = code
		if err := validateCanton(entry); err != nil {
			return nil, fmt.Errorf("plate_cantons.json: canton %s: %w", code, err)
		}
		out[code] = entry
	}
	for code := range ValidCantons {
		if _, ok := out[code]; !ok {
			return nil, fmt.Errorf("plate_cantons.json: missing canton %s", code)
		}
	}
	return out, nil
}

// loadCantonsFromBytes is a test helper that bypasses the sync.Once guard.
func loadCantonsFromBytes(raw []byte) (map[string]CantonEntry, error) {
	return decodeCantons(raw)
}

func validateCanton(c CantonEntry) error {
	// Names must include at least 'de'.
	if c.Names == nil || strings.TrimSpace(c.Names["de"]) == "" {
		return fmt.Errorf("names.de is required")
	}

	// Authority name required.
	if strings.TrimSpace(c.Authority.Name) == "" {
		return fmt.Errorf("authority.name is required")
	}

	h := c.Halterauskunft
	switch h.Mode {
	case ModeOnline, ModePostal, ModeMixed, ModeUnavailable:
		// ok
	default:
		return fmt.Errorf("halterauskunft.mode %q is invalid", h.Mode)
	}

	if h.Mode == ModeOnline || h.Mode == ModeMixed {
		if strings.TrimSpace(h.URL) == "" {
			return fmt.Errorf("halterauskunft.url is required when mode=%s", h.Mode)
		}
	}
	if h.Mode == ModePostal || h.Mode == ModeMixed {
		if c.Authority.Postal.IsEmpty() {
			return fmt.Errorf("authority.postal must be populated when mode=%s", h.Mode)
		}
	}
	if h.CostCHF < 0 {
		return fmt.Errorf("halterauskunft.cost_chf must be >= 0, got %v", h.CostCHF)
	}

	// Validate enums that have fixed values.
	switch h.Auth.Captcha {
	case CaptchaNone, CaptchaHCaptcha, CaptchaReCaptcha, "":
		// ok; empty allowed for unavailable-mode stubs
	default:
		return fmt.Errorf("halterauskunft.auth.captcha %q is invalid", h.Auth.Captcha)
	}
	switch h.Processing.Typical {
	case ProcInstant, ProcHours, Proc1to3, Proc5to10, "":
	default:
		return fmt.Errorf("halterauskunft.processing.typical %q is invalid", h.Processing.Typical)
	}
	switch h.Processing.Delivery {
	case DeliveryPDFEmail, DeliveryPostal, DeliveryOnlinePortal, "":
	default:
		return fmt.Errorf("halterauskunft.processing.delivery %q is invalid", h.Processing.Delivery)
	}

	// Verification.
	if _, err := time.Parse("2006-01-02", c.Verification.LastVerified); err != nil {
		return fmt.Errorf("verification.last_verified %q is not a YYYY-MM-DD date: %w", c.Verification.LastVerified, err)
	}
	switch c.Verification.VerifiedBy {
	case VerifiedManual, VerifiedCI:
		// ok
	default:
		return fmt.Errorf("verification.verified_by %q is invalid", c.Verification.VerifiedBy)
	}

	return nil
}

// AllCantons returns the canton entries sorted by code. Useful for verify.
func AllCantons() ([]CantonEntry, error) {
	m, err := LoadCantons()
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(m))
	for code := range m {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]CantonEntry, 0, len(codes))
	for _, c := range codes {
		out = append(out, m[c])
	}
	return out, nil
}
