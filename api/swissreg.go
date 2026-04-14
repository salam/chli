package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Defaults and constants for the public swissreg.ch search backend.
const (
	swissregBaseHost = "https://www.swissreg.ch"
	swissregSearch   = "/database/resources/query/search"
	swissregImage    = "/database/resources/ds/image/"
	swissregIPIVers  = "10.0.9"
	swissregCacheTTL = 6 * time.Hour

	// Hard upper bound enforced by the backend; requests above this are silently
	// capped (experimentally verified). Exposed so the CLI can communicate it.
	SwissregMaxPageSize = 64

	// Image format hashes carried on result rows.
	SwissregImgScreen    = "bild_trefferliste_screen_hash__type_string"
	SwissregImgPrint     = "bild_trefferliste_print_hash__type_string"
	SwissregImgThumbnail = "bild_trefferliste_thumbnail_hash__type_string"
)

// swissregBaseHostOverride lets tests redirect requests at an httptest server.
// Left empty in production builds; see api/swissreg_test.go.
var swissregBaseHostOverride string

func swissregHost() string {
	if swissregBaseHostOverride != "" {
		return swissregBaseHostOverride
	}
	return swissregBaseHost
}

// Valid `target` values accepted by the public search backend.
var SwissregTargets = []string{
	"chmarke",           // CH & IR trademarks (server returns both)
	"patent",            // CH & EP patents
	"design",            // designs
	"publikationpatent", // patent publications
	"publikationdesign", // design publications
}

// SwissregQuery bundles the parameters of a search request.
type SwissregQuery struct {
	Target       string
	SearchString string
	Filters      map[string][]string
	PageSize     int
}

type swissregReqBody struct {
	Target       string              `json:"target"`
	SearchString string              `json:"searchString"`
	Filters      map[string][]string `json:"filters"`
	SortByField  string              `json:"sortByField,omitempty"`
	SortOrder    string              `json:"sortOrder,omitempty"`
	PageSize     int                 `json:"pageSize"`
}

// SwissregSearch runs a query against the public swissreg search backend.
// Quote the query string ("foo bar") for exact-phrase matching.
func (c *Client) SwissregSearch(q SwissregQuery) (*SwissregSearchResponse, error) {
	if q.PageSize <= 0 {
		q.PageSize = 16
	}
	if q.PageSize > SwissregMaxPageSize {
		q.PageSize = SwissregMaxPageSize
	}
	if q.Filters == nil {
		q.Filters = map[string][]string{}
	}
	body := swissregReqBody{
		Target:       q.Target,
		SearchString: q.SearchString,
		Filters:      q.Filters,
		SortByField:  "score",
		SortOrder:    "DESC",
		PageSize:     q.PageSize,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	endpoint := swissregHost() + swissregSearch
	cacheKey := "POST:" + endpoint + ":" + string(bodyBytes)
	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			var out SwissregSearchResponse
			if err := json.Unmarshal(data, &out); err == nil {
				return &out, nil
			}
		}
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-ipi-version", swissregIPIVers)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(string(bodyBytes))), nil
	}

	resp, err := c.doWithRetry(req, 2)
	if err != nil {
		return nil, fmt.Errorf("could not reach swissreg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("swissreg: unknown target %q (valid: %s)", q.Target, strings.Join(SwissregTargets, ", "))
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("swissreg API error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var out SwissregSearchResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	c.Cache.Set(cacheKey, data, swissregCacheTTL)
	return &out, nil
}

// SwissregDetail fetches a single record by its internal URN id (filters by id).
// Accepts either the full URN ("urn:ige:schutztitel:chmarke:1206422825") or,
// if the target is known, a bare internal id. Returns nil when not found.
func (c *Client) SwissregDetail(target, id string) (SwissregResult, error) {
	urn := id
	if !strings.HasPrefix(urn, "urn:ige:") {
		urn = "urn:ige:schutztitel:" + target + ":" + id
	}
	resp, err := c.SwissregSearch(SwissregQuery{
		Target:   target,
		Filters:  map[string][]string{"id": {urn}},
		PageSize: 1,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, nil
	}
	return resp.Results[0], nil
}

// SwissregImageURL builds the public image URL for a trademark image hash.
// Format must be one of "screen" (default), "print", "thumbnail".
// Swissreg ignores the format in the URL path — all three hashes point at the
// same service; we return a ready-to-use link.
func SwissregImageURL(hash string) string {
	if hash == "" {
		return ""
	}
	return swissregHost() + swissregImage + "urn:ige:img:" + hash
}

// SwissregFetchImage downloads a trademark image by hash. Requires a browser-
// like User-Agent + Referer to clear the WAF.
func (c *Client) SwissregFetchImage(hash string) (data []byte, contentType string, err error) {
	if hash == "" {
		return nil, "", fmt.Errorf("empty image hash")
	}
	u := SwissregImageURL(hash)
	// Validate to catch header-injection style ids.
	if _, perr := url.Parse(u); perr != nil {
		return nil, "", fmt.Errorf("bad image url: %w", perr)
	}
	headers := map[string]string{
		"Referer":       swissregHost() + "/database-client/search",
		"x-ipi-version": swissregIPIVers,
		"Accept":        "image/*",
	}
	return c.DownloadFile(u, headers)
}
