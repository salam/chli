package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"
)

const (
	lindasEndpoint = "https://lindas.admin.ch/query"
	lindasCacheTTL = 24 * time.Hour
)

// LindasSPARQL executes a raw SPARQL query against the LINDAS endpoint
// (linked data covering IPI, federal statistics, procurement, energy, etc.).
func (c *Client) LindasSPARQL(query string) (*SPARQLResult, error) {
	body := "query=" + url.QueryEscape(query)
	cacheKey := "POST:" + lindasEndpoint + ":" + body

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			var r SPARQLResult
			if err := json.Unmarshal(data, &r); err == nil {
				return &r, nil
			}
		}
	}

	req, err := newPostRequest(lindasEndpoint, body)
	if err != nil {
		return nil, err
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, fmt.Errorf("could not reach LINDAS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LINDAS API error %d: %s", resp.StatusCode, string(b))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set(cacheKey, data, lindasCacheTTL)

	var result SPARQLResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing SPARQL response: %w", err)
	}
	return &result, nil
}

// LindasDatasets lists the datasets (named graphs) exposed by LINDAS.
func (c *Client) LindasDatasets() (*SPARQLResult, error) {
	const q = `
PREFIX dcat: <http://www.w3.org/ns/dcat#>
PREFIX dct:  <http://purl.org/dc/terms/>
SELECT DISTINCT ?dataset ?title WHERE {
  ?dataset a dcat:Dataset ;
           dct:title ?title .
  FILTER(LANG(?title) = "de" || LANG(?title) = "")
} ORDER BY ?title LIMIT 200`
	return c.LindasSPARQL(q)
}
