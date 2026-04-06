package api

import (
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
