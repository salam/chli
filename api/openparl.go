package api

import (
	"fmt"
	"net/url"
	"time"
)

const openparlBaseURL = "https://api.openparldata.ch/v1/"

// openParlPersonID resolves a parliament PersonNumber to an OpenParlData person_id
// by checking the access_badges endpoint (where person_external_id filtering works).
func (c *Client) openParlPersonID(personNumber int) (int, error) {
	// Try via access_badges first (person_external_id filter works here)
	path := fmt.Sprintf("access_badges/?person_external_id=%d&limit=1", personNumber)
	var resp OpenParlResponse[OpenParlAccessBadge]
	if err := c.DoJSONWithTTL(openparlBaseURL, path, &resp, 24*time.Hour); err != nil {
		return 0, err
	}
	if len(resp.Data) > 0 {
		return resp.Data[0].PersonID, nil
	}
	// If no badges, try searching persons by name — caller will handle 0
	return 0, fmt.Errorf("person not found in OpenParlData")
}

// OpenParlInterests fetches declared interests for a parliament member by their PersonNumber.
func (c *Client) OpenParlInterests(personNumber int) ([]OpenParlInterest, error) {
	pid, err := c.openParlPersonID(personNumber)
	if err != nil {
		return nil, nil // silently return empty if person not in OpenParlData
	}
	path := fmt.Sprintf("persons/%d/interests?limit=100", pid)
	var resp OpenParlResponse[OpenParlInterest]
	if err := c.DoJSONWithTTL(openparlBaseURL, path, &resp, 24*time.Hour); err != nil {
		return nil, fmt.Errorf("openparldata interests: %w", err)
	}
	return resp.Data, nil
}

// OpenParlAccessBadges fetches lobby access badges for a parliament member by their PersonNumber.
func (c *Client) OpenParlAccessBadges(personNumber int) ([]OpenParlAccessBadge, error) {
	path := fmt.Sprintf("access_badges/?person_external_id=%d&limit=100", personNumber)
	var resp OpenParlResponse[OpenParlAccessBadge]
	if err := c.DoJSONWithTTL(openparlBaseURL, path, &resp, 24*time.Hour); err != nil {
		return nil, fmt.Errorf("openparldata access badges: %w", err)
	}
	return resp.Data, nil
}

// OpenParlSearchInterests searches interests by name (e.g. company name)
// and resolves person names.
func (c *Client) OpenParlSearchInterests(query string, limit int) ([]OpenParlInterest, error) {
	if limit <= 0 {
		limit = 50
	}
	path := fmt.Sprintf("interests/?search=%s&limit=%d", url.QueryEscape(query), limit)
	var resp OpenParlResponse[OpenParlInterest]
	if err := c.DoJSONWithTTL(openparlBaseURL, path, &resp, 24*time.Hour); err != nil {
		return nil, fmt.Errorf("openparldata interest search: %w", err)
	}

	// Resolve person names for unique person_ids
	personIDs := make(map[int]bool)
	for _, interest := range resp.Data {
		personIDs[interest.PersonID] = true
	}
	personNames := make(map[int]string)
	for pid := range personIDs {
		var person OpenParlPerson
		ppath := fmt.Sprintf("persons/%d", pid)
		if err := c.DoJSONWithTTL(openparlBaseURL, ppath, &person, 24*time.Hour); err == nil {
			personNames[pid] = person.Fullname
		}
	}
	for i := range resp.Data {
		if name, ok := personNames[resp.Data[i].PersonID]; ok {
			resp.Data[i].PersonFullname = name
		}
	}

	return resp.Data, nil
}
