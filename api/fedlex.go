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

const (
	fedlexEndpoint = "https://fedlex.data.admin.ch/sparqlendpoint"
	fedlexCacheTTL = 168 * time.Hour // 7 days
)

// LangURI maps a short language code to the Fedlex language URI suffix.
func LangURI(lang string) string {
	switch lang {
	case "fr":
		return "FRA"
	case "it":
		return "ITA"
	case "en":
		return "ENG"
	case "rm":
		return "ROH"
	default:
		return "DEU"
	}
}

// FedlexSPARQL executes a SPARQL query against the Fedlex endpoint.
func (c *Client) FedlexSPARQL(query string) (*SPARQLResult, error) {
	body := "query=" + url.QueryEscape(query)
	cacheKey := "POST:" + fedlexEndpoint + ":" + body

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			var result SPARQLResult
			if err := json.Unmarshal(data, &result); err != nil {
				return nil, fmt.Errorf("parsing cached response: %w", err)
			}
			return &result, nil
		}
	}

	req, err := newPostRequest(fedlexEndpoint, body)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Fedlex: check your internet connection")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Fedlex API error %d: %s", resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	c.Cache.Set(cacheKey, data, fedlexCacheTTL)

	var result SPARQLResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing SPARQL response: %w", err)
	}
	return &result, nil
}

// FedlexSR looks up consolidated laws by SR number.
func (c *Client) FedlexSR(number string, lang string) ([]SREntry, error) {
	query := fmt.Sprintf(QuerySRByNumber, number, LangURI(lang))
	result, err := c.FedlexSPARQL(query)
	if err != nil {
		return nil, err
	}
	return parseSREntries(result), nil
}

// FedlexSearch searches for laws by title substring.
func (c *Client) FedlexSearch(searchTerm string, lang string) ([]FedlexSearchResult, error) {
	query := fmt.Sprintf(QuerySearchTitle, LangURI(lang), searchTerm)
	result, err := c.FedlexSPARQL(query)
	if err != nil {
		return nil, err
	}
	return parseSearchResults(result), nil
}

// FedlexBBL fetches Federal Gazette entries for a given year.
func (c *Client) FedlexBBL(year string, lang string) ([]BBLEntry, error) {
	query := fmt.Sprintf(QueryBBLByYear, LangURI(lang), year)
	result, err := c.FedlexSPARQL(query)
	if err != nil {
		return nil, err
	}
	return parseBBLEntries(result), nil
}

// FedlexConsultations fetches consultations with an optional status filter.
func (c *Client) FedlexConsultations(status string, lang string) ([]ConsultationEntry, error) {
	filter := ""
	if status != "" {
		filter = fmt.Sprintf(`FILTER(CONTAINS(LCASE(STR(?status)), LCASE("%s")))`, status)
	}
	query := fmt.Sprintf(QueryConsultations, LangURI(lang), filter)
	result, err := c.FedlexSPARQL(query)
	if err != nil {
		return nil, err
	}
	return parseConsultationEntries(result), nil
}

// FedlexTreaties fetches treaties with optional partner and year filters.
func (c *Client) FedlexTreaties(partner string, year string, lang string) ([]TreatyEntry, error) {
	var filters []string
	if partner != "" {
		filters = append(filters, fmt.Sprintf(`FILTER(CONTAINS(LCASE(STR(?partner)), LCASE("%s")))`, partner))
	}
	if year != "" {
		filters = append(filters, fmt.Sprintf(`FILTER(STRSTARTS(STR(?dateDoc), "%s"))`, year))
	}
	filter := strings.Join(filters, "\n  ")
	query := fmt.Sprintf(QueryTreaties, LangURI(lang), filter)
	result, err := c.FedlexSPARQL(query)
	if err != nil {
		return nil, err
	}
	return parseTreatyEntries(result), nil
}

// newPostRequest builds an HTTP POST request for the SPARQL endpoint
// with the correct Content-Type and Accept headers.
func newPostRequest(endpoint, body string) (*http.Request, error) {
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/sparql-results+json")
	return req, nil
}

func parseSREntries(result *SPARQLResult) []SREntry {
	seen := make(map[string]bool)
	entries := make([]SREntry, 0, len(result.Results.Bindings))
	for _, b := range result.Results.Bindings {
		uri := val(b, "uri")
		if seen[uri] {
			continue
		}
		seen[uri] = true
		label := val(b, "inForceLabel")
		if label == "" {
			label = shortenURI(val(b, "inForceURI"))
		}
		entries = append(entries, SREntry{
			URI:          uri,
			Title:        val(b, "title"),
			DateDoc:      val(b, "dateDoc"),
			InForceURI:   val(b, "inForceURI"),
			InForceLabel: label,
		})
	}
	return entries
}

func parseSearchResults(result *SPARQLResult) []FedlexSearchResult {
	entries := make([]FedlexSearchResult, 0, len(result.Results.Bindings))
	for _, b := range result.Results.Bindings {
		entries = append(entries, FedlexSearchResult{
			URI:        val(b, "uri"),
			Identifier: val(b, "identifier"),
			Title:      val(b, "title"),
			DateDoc:    val(b, "dateDoc"),
			InForce:    shortenURI(val(b, "inForce")),
		})
	}
	return entries
}

func parseBBLEntries(result *SPARQLResult) []BBLEntry {
	entries := make([]BBLEntry, 0, len(result.Results.Bindings))
	for _, b := range result.Results.Bindings {
		entries = append(entries, BBLEntry{
			URI:     val(b, "uri"),
			Title:   val(b, "title"),
			DateDoc: val(b, "dateDoc"),
		})
	}
	return entries
}

func parseConsultationEntries(result *SPARQLResult) []ConsultationEntry {
	entries := make([]ConsultationEntry, 0, len(result.Results.Bindings))
	for _, b := range result.Results.Bindings {
		entries = append(entries, ConsultationEntry{
			URI:     val(b, "uri"),
			Title:   val(b, "title"),
			DateDoc: val(b, "dateDoc"),
			Status:  shortenURI(val(b, "status")),
		})
	}
	return entries
}

func parseTreatyEntries(result *SPARQLResult) []TreatyEntry {
	entries := make([]TreatyEntry, 0, len(result.Results.Bindings))
	for _, b := range result.Results.Bindings {
		entries = append(entries, TreatyEntry{
			URI:     val(b, "uri"),
			Title:   val(b, "title"),
			DateDoc: val(b, "dateDoc"),
			Partner: shortenURI(val(b, "partner")),
		})
	}
	return entries
}

func val(binding map[string]SPARQLValue, key string) string {
	if v, ok := binding[key]; ok {
		return v.Value
	}
	return ""
}

// shortenURI extracts the last path segment from a URI for display.
func shortenURI(uri string) string {
	if uri == "" {
		return ""
	}
	if i := strings.LastIndex(uri, "/"); i >= 0 && i < len(uri)-1 {
		return uri[i+1:]
	}
	if i := strings.LastIndex(uri, "#"); i >= 0 && i < len(uri)-1 {
		return uri[i+1:]
	}
	return uri
}
