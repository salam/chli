package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/matthiasak/chli/output"
)

// The OData service at ws.parlament.ch does not expose federal departments,
// so we fall back to the legacy JSON endpoint for this one resource.
const parlOldBaseURL = "https://ws-old.parlament.ch"

// ParlDepartment represents a federal department as returned by
// ws-old.parlament.ch/departments (current) or /departments/historic.
// From/To are only populated for historic records.
type ParlDepartment struct {
	ID           int    `json:"id"`
	Updated      string `json:"updated"`
	Abbreviation string `json:"abbreviation"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
}

// oldLangCode maps output.Lang to the ws-old.parlament.ch lang query value.
func oldLangCode() string {
	switch strings.ToLower(output.Lang) {
	case "fr", "it", "en":
		return strings.ToLower(output.Lang)
	default:
		return "de"
	}
}

// ParlDepartments fetches the current list of federal departments.
// If historic is true, fetches /departments/historic which includes
// end-dated records with From/To fields.
func (c *Client) ParlDepartments(historic bool) ([]ParlDepartment, error) {
	path := "/departments"
	if historic {
		path = "/departments/historic"
	}
	v := url.Values{}
	v.Set("format", "json")
	v.Set("lang", oldLangCode())
	path = path + "?" + v.Encode()

	// ws-old.parlament.ch sits behind Akamai, which 403s HTTP/1.1 requests
	// that claim a modern Chrome User-Agent. A plain curl-style UA is accepted.
	headers := map[string]string{
		"User-Agent": "curl/8.4.0",
		"Accept":     "application/json",
	}
	var result []ParlDepartment
	if err := c.DoGetWithHeaders(parlOldBaseURL+path, headers, &result, 24*time.Hour); err != nil {
		return nil, fmt.Errorf("fetching departments: %w", err)
	}
	return result, nil
}
