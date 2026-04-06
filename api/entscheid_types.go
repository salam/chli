package api

import "github.com/matthiasak/chli/output"

// ESResponse is the top-level Elasticsearch response from entscheidsuche.ch.
type ESResponse struct {
	Hits ESHits `json:"hits"`
}

// ESHits holds the hits wrapper with total count and individual hits.
type ESHits struct {
	Total ESTotal `json:"total"`
	Hits  []ESHit `json:"hits"`
}

// ESTotal describes how many results matched.
type ESTotal struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"`
}

// ESHit is a single search result.
type ESHit struct {
	Index  string     `json:"_index"`
	ID     string     `json:"_id"`
	Score  float64    `json:"_score"`
	Source ESDecision `json:"_source"`
}

// ESDecision holds the fields of a court decision document.
type ESDecision struct {
	ID         string                  `json:"id"`
	Date       string                  `json:"date"`
	Canton     string                  `json:"canton"`
	Hierarchy  []string                `json:"hierarchy"`
	Title      output.MultilingualText `json:"title"`
	Abstract   output.MultilingualText `json:"abstract"`
	Reference  []string                `json:"reference"`
	Attachment ESAttachment            `json:"attachment"`
	Meta       output.MultilingualText `json:"meta"`
	ScrapeDate string                  `json:"scrapedate"`
	Source     string                  `json:"source"`
}

// ESAttachment holds the document attachment info (typically a PDF link).
type ESAttachment struct {
	ContentURL  string `json:"content_url"`
	ContentType string `json:"content_type"`
}

// Court describes a known court for filtering.
type Court struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
