package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

const opendataBaseURL = "https://ckan.opendata.swiss/api/3/action/"
const opendataTTL = 24 * time.Hour

// OpendataSearch searches datasets on opendata.swiss.
// org and format are optional filters (pass "" to skip).
func (c *Client) OpendataSearch(query string, org string, format string, rows, start int) (*CKANSearchResult, error) {
	q := query
	if org != "" {
		q += " organization:" + org
	}
	if format != "" {
		q += " res_format:" + format
	}

	path := "package_search?" + url.Values{
		"q":     {q},
		"rows":  {strconv.Itoa(rows)},
		"start": {strconv.Itoa(start)},
	}.Encode()

	var resp CKANResponse
	if err := c.DoJSONWithTTL(opendataBaseURL, path, &resp, opendataTTL); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("CKAN API returned success=false")
	}

	var result CKANSearchResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parsing search result: %w", err)
	}
	return &result, nil
}

// OpendataDataset fetches a single dataset by name or ID.
func (c *Client) OpendataDataset(id string) (*CKANDataset, error) {
	path := "package_show?" + url.Values{
		"id": {id},
	}.Encode()

	var resp CKANResponse
	if err := c.DoJSONWithTTL(opendataBaseURL, path, &resp, opendataTTL); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("CKAN API returned success=false")
	}

	var dataset CKANDataset
	if err := json.Unmarshal(resp.Result, &dataset); err != nil {
		return nil, fmt.Errorf("parsing dataset: %w", err)
	}
	return &dataset, nil
}

// OpendataOrgs fetches the list of organizations from opendata.swiss.
func (c *Client) OpendataOrgs() ([]CKANOrg, error) {
	path := "organization_list?" + url.Values{
		"all_fields": {"true"},
		"limit":      {"1000"},
	}.Encode()

	var resp CKANResponse
	if err := c.DoJSONWithTTL(opendataBaseURL, path, &resp, opendataTTL); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("CKAN API returned success=false")
	}

	var orgs []CKANOrg
	if err := json.Unmarshal(resp.Result, &orgs); err != nil {
		return nil, fmt.Errorf("parsing organizations: %w", err)
	}
	return orgs, nil
}
