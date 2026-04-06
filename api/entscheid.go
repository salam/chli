package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const entscheidBaseURL = "https://entscheidsuche.ch/"
const entscheidCacheTTL = 24 * time.Hour

// EntscheidSearch searches court decisions via the entscheidsuche.ch Elasticsearch API.
func (c *Client) EntscheidSearch(query string, court string, dateFrom string, dateTo string, size int) (*ESResponse, error) {
	esQuery := buildSearchQuery(query, court, dateFrom, dateTo, size)

	body, err := json.Marshal(esQuery)
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	var result ESResponse
	if err := c.DoPost(entscheidBaseURL, "_search.php", "application/json", string(body), &result, entscheidCacheTTL); err != nil {
		return nil, fmt.Errorf("entscheid search: %w", err)
	}
	return &result, nil
}

// EntscheidGet fetches a single decision by its exact ID.
func (c *Client) EntscheidGet(id string) (*ESDecision, error) {
	esQuery := map[string]any{
		"query": map[string]any{
			"term": map[string]any{
				"id": id,
			},
		},
		"size": 1,
	}

	body, err := json.Marshal(esQuery)
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	var result ESResponse
	if err := c.DoPost(entscheidBaseURL, "_search.php", "application/json", string(body), &result, entscheidCacheTTL); err != nil {
		return nil, fmt.Errorf("entscheid get: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return nil, fmt.Errorf("decision not found: %s", id)
	}
	return &result.Hits.Hits[0].Source, nil
}

// EntscheidCourts returns the list of known courts and cantons.
func EntscheidCourts() []Court {
	return []Court{
		{"BGer", "Bundesgericht", "Federal Supreme Court"},
		{"BVGer", "Bundesverwaltungsgericht", "Federal Administrative Court"},
		{"BStGer", "Bundesstrafgericht", "Federal Criminal Court"},
		{"AG", "Aargau", "Canton Aargau"},
		{"AI", "Appenzell Innerrhoden", "Canton Appenzell Innerrhoden"},
		{"AR", "Appenzell Ausserrhoden", "Canton Appenzell Ausserrhoden"},
		{"BE", "Bern", "Canton Bern"},
		{"BL", "Basel-Landschaft", "Canton Basel-Landschaft"},
		{"BS", "Basel-Stadt", "Canton Basel-Stadt"},
		{"FR", "Fribourg", "Canton Fribourg"},
		{"GE", "Genève", "Canton Geneva"},
		{"GL", "Glarus", "Canton Glarus"},
		{"GR", "Graubünden", "Canton Graubünden"},
		{"JU", "Jura", "Canton Jura"},
		{"LU", "Luzern", "Canton Lucerne"},
		{"NE", "Neuchâtel", "Canton Neuchâtel"},
		{"NW", "Nidwalden", "Canton Nidwalden"},
		{"OW", "Obwalden", "Canton Obwalden"},
		{"SG", "St. Gallen", "Canton St. Gallen"},
		{"SH", "Schaffhausen", "Canton Schaffhausen"},
		{"SO", "Solothurn", "Canton Solothurn"},
		{"SZ", "Schwyz", "Canton Schwyz"},
		{"TG", "Thurgau", "Canton Thurgau"},
		{"TI", "Ticino", "Canton Ticino"},
		{"UR", "Uri", "Canton Uri"},
		{"VD", "Vaud", "Canton Vaud"},
		{"VS", "Valais", "Canton Valais"},
		{"ZG", "Zug", "Canton Zug"},
		{"ZH", "Zürich", "Canton Zürich"},
	}
}

// buildSearchQuery constructs the Elasticsearch query body.
func buildSearchQuery(query string, court string, dateFrom string, dateTo string, size int) map[string]any {
	var musts []map[string]any
	var filters []map[string]any

	if query != "" {
		musts = append(musts, map[string]any{
			"query_string": map[string]any{
				"query":  query,
				"fields": []string{"abstract.de", "abstract.fr", "title.de", "title.fr", "reference"},
			},
		})
	}

	if dateFrom != "" || dateTo != "" {
		dateRange := map[string]any{}
		if dateFrom != "" {
			dateRange["gte"] = dateFrom
		}
		if dateTo != "" {
			dateRange["lte"] = dateTo
		}
		filters = append(filters, map[string]any{
			"range": map[string]any{
				"date": dateRange,
			},
		})
	}

	if court != "" {
		courtUpper := strings.ToUpper(court)
		filters = append(filters, map[string]any{
			"term": map[string]any{
				"canton": courtUpper,
			},
		})
	}

	boolQuery := map[string]any{}
	if len(musts) > 0 {
		boolQuery["must"] = musts
	}
	if len(filters) > 0 {
		boolQuery["filter"] = filters
	}

	var queryClause map[string]any
	if len(boolQuery) > 0 {
		queryClause = map[string]any{"bool": boolQuery}
	} else {
		queryClause = map[string]any{"match_all": map[string]any{}}
	}

	return map[string]any{
		"query":   queryClause,
		"from":    0,
		"size":    size,
		"_source": map[string]any{"excludes": []string{"attachment.content"}},
	}
}
