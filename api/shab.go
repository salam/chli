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
