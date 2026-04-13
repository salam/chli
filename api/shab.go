package api

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const shabBaseURL = "https://shab.ch/api/v1/"
const shabCacheTTL = 1 * time.Hour

// SHABPublicationURL returns the public shab.ch detail URL for a publication UUID.
func SHABPublicationURL(uuid string) string {
	if uuid == "" {
		return ""
	}
	return "https://shab.ch/#!/search/publications/detail/" + uuid
}

// shabHeaders returns the headers required for all SHAB API requests.
func shabHeaders() map[string]string {
	return map[string]string{
		"x-requested-with": "XMLHttpRequest",
		"Accept":           "application/json",
	}
}

// SHABSearch searches SHAB publications by keyword and optional rubric filter.
func (c *Client) SHABSearch(keyword string, rubrics []string, page, size int) (*SHABSearchResult, error) {
	params := url.Values{}
	if keyword != "" {
		params.Set("keyword", keyword)
	}
	if len(rubrics) > 0 {
		params.Set("rubrics", strings.Join(rubrics, ","))
	}
	params.Set("publicationStates", "PUBLISHED,CANCELLED")
	params.Set("pageRequest.page", fmt.Sprintf("%d", page))
	params.Set("pageRequest.size", fmt.Sprintf("%d", size))
	params.Set("includeContent", "false")
	params.Set("allowRubricSelection", "false")

	u := shabBaseURL + "publications?" + params.Encode()

	var result SHABSearchResult
	if err := c.DoGetWithHeaders(u, shabHeaders(), &result, shabCacheTTL); err != nil {
		return nil, fmt.Errorf("SHAB search: %w", err)
	}
	return &result, nil
}

// SHABResolveID resolves a publication number (e.g. "HR02-1005024497") to its UUID
// by searching the SHAB API. If the input already looks like a UUID, it is returned as-is.
func (c *Client) SHABResolveID(idOrNumber string) (string, error) {
	// UUIDs contain lowercase hex and dashes in 8-4-4-4-12 pattern; publication numbers don't
	if len(idOrNumber) == 36 && idOrNumber[8] == '-' && idOrNumber[13] == '-' {
		return idOrNumber, nil
	}
	// Search by publication number
	result, err := c.SHABSearch(idOrNumber, nil, 0, 5)
	if err != nil {
		return "", fmt.Errorf("resolving publication number %s: %w", idOrNumber, err)
	}
	for _, pub := range result.Content {
		if pub.Meta.PublicationNumber == idOrNumber {
			return pub.Meta.ID, nil
		}
	}
	return "", fmt.Errorf("no publication found for number %s", idOrNumber)
}

// SHABPublication fetches the XML detail for a single publication by UUID.
func (c *Client) SHABPublication(id string) ([]byte, error) {
	u := shabBaseURL + "publications/" + url.PathEscape(id) + "/xml"
	headers := shabHeaders()
	headers["Accept"] = "application/xml"

	data, err := c.DoGetRawWithHeaders(u, headers, shabCacheTTL)
	if err != nil {
		return nil, fmt.Errorf("SHAB publication %s: %w", id, err)
	}
	return data, nil
}

// SHABPublicationParsed fetches the XML for a publication and attempts to parse it.
// It returns the parsed struct, the raw bytes, and any parse error.
// If parsing fails the caller can fall back to raw XML display.
func (c *Client) SHABPublicationParsed(id string) (*SHABPublicationXML, []byte, error) {
	data, err := c.SHABPublication(id)
	if err != nil {
		return nil, nil, err
	}
	var pub SHABPublicationXML
	if xmlErr := xml.Unmarshal(data, &pub); xmlErr != nil {
		return nil, data, nil // parse failed, return raw bytes
	}
	return &pub, data, nil
}

// SHABHistory walks the lastFosc back-pointer chain starting from id and
// returns the publications newest-first. If depth > 0 the walk stops after
// that many hops. Unresolved references terminate the walk silently.
func (c *Client) SHABHistory(id string, depth int) ([]*SHABPublicationXML, error) {
	var chain []*SHABPublicationXML
	visited := map[string]bool{}
	curID := id
	hops := 0
	for curID != "" {
		if visited[curID] {
			break // guard against cycles
		}
		visited[curID] = true
		pub, _, err := c.SHABPublicationParsed(curID)
		if err != nil {
			if len(chain) == 0 {
				return nil, err
			}
			break
		}
		if pub == nil {
			break
		}
		chain = append(chain, pub)
		if pub.Content.LastFosc == nil || pub.Content.LastFosc.Sequence == "" {
			break
		}
		if depth > 0 && hops+1 >= depth {
			break
		}
		hops++
		next, err := c.SHABResolveID(pub.Content.LastFosc.Sequence)
		if err != nil || next == "" {
			break
		}
		curID = next
	}
	return chain, nil
}
