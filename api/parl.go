package api

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/matthiasak/chli/output"
)

const parlBaseURL = "https://ws.parlament.ch/odata.svc/"

// ODataQuery builds OData v3 query URLs for the Parliament API.
type ODataQuery struct {
	table   string
	filters []string
	selects []string
	top     int
	skip    int
	orderBy string
	expands []string
}

// NewODataQuery creates a new query builder for the given entity set (table).
func NewODataQuery(table string) *ODataQuery {
	return &ODataQuery{table: table}
}

// Filter adds a $filter clause (combined with 'and').
func (q *ODataQuery) Filter(f string) *ODataQuery {
	if f != "" {
		q.filters = append(q.filters, f)
	}
	return q
}

// Select specifies $select fields.
func (q *ODataQuery) Select(fields ...string) *ODataQuery {
	q.selects = append(q.selects, fields...)
	return q
}

// Top sets $top (max results).
func (q *ODataQuery) Top(n int) *ODataQuery {
	q.top = n
	return q
}

// Skip sets $skip for pagination.
func (q *ODataQuery) Skip(n int) *ODataQuery {
	q.skip = n
	return q
}

// OrderBy sets $orderby.
func (q *ODataQuery) OrderBy(field string) *ODataQuery {
	q.orderBy = field
	return q
}

// Expand adds $expand navigation properties.
func (q *ODataQuery) Expand(nav ...string) *ODataQuery {
	q.expands = append(q.expands, nav...)
	return q
}

// langCode maps output.Lang to the OData Language filter value.
func langCode() string {
	switch strings.ToLower(output.Lang) {
	case "fr":
		return "FR"
	case "it":
		return "IT"
	case "en":
		return "EN"
	default:
		return "DE"
	}
}

// Build produces the URL path with properly encoded query parameters.
func (q *ODataQuery) Build() string {
	v := url.Values{}
	v.Set("$format", "json")

	// Always filter by language.
	allFilters := append([]string{fmt.Sprintf("Language eq '%s'", langCode())}, q.filters...)
	v.Set("$filter", strings.Join(allFilters, " and "))

	if len(q.selects) > 0 {
		v.Set("$select", strings.Join(q.selects, ","))
	}
	if q.top > 0 {
		v.Set("$top", strconv.Itoa(q.top))
	}
	if q.skip > 0 {
		v.Set("$skip", strconv.Itoa(q.skip))
	}
	if q.orderBy != "" {
		v.Set("$orderby", q.orderBy)
	}
	if len(q.expands) > 0 {
		v.Set("$expand", strings.Join(q.expands, ","))
	}

	return q.table + "?" + v.Encode()
}

// parlCurl fetches a URL using curl. The Parliament WAF uses TLS fingerprinting
// that blocks Go's net/http client, so we shell out to curl.
func (c *Client) parlCurl(url string) ([]byte, error) {
	cacheKey := "GET:" + url

	if !c.NoCache && !c.Refresh {
		if data, ok := c.Cache.Get(cacheKey); ok {
			return data, nil
		}
	}

	out, err := exec.Command("curl", "-sf", "--max-time", "60", url).Output()
	if err != nil {
		return nil, fmt.Errorf("parliament API: request failed (curl): %w", err)
	}

	c.Cache.Set(cacheKey, out, 1*time.Hour)
	return out, nil
}

// ParlQuery executes an OData query and returns the raw JSON array from the "d" field.
func (c *Client) ParlQuery(q *ODataQuery) (json.RawMessage, error) {
	fullURL := parlBaseURL + q.Build()

	data, err := c.parlCurl(fullURL)
	if err != nil {
		return nil, err
	}

	var odataResp ODataResponse
	if err := json.Unmarshal(data, &odataResp); err != nil {
		return nil, fmt.Errorf("parliament API: parsing response: %w", err)
	}
	return odataResp.D, nil
}

// odataResultsWrapper handles the OData v3 {"results": [...]} envelope.
type odataResultsWrapper struct {
	Results json.RawMessage `json:"results"`
}

// ParlQueryInto executes an OData query and unmarshals the result into a typed slice.
func (c *Client) ParlQueryInto(q *ODataQuery, result any) error {
	raw, err := c.ParlQuery(q)
	if err != nil {
		return err
	}

	// OData v3 returns "d" as either:
	//   1. [...] — plain array
	//   2. {"results": [...]} — object with results wrapper
	// Try plain array first.
	if err := json.Unmarshal(raw, result); err != nil {
		// Try {"results": [...]} wrapper
		var wrapper odataResultsWrapper
		if err2 := json.Unmarshal(raw, &wrapper); err2 == nil && len(wrapper.Results) > 0 {
			if err3 := json.Unmarshal(wrapper.Results, result); err3 == nil {
				return nil
			}
		}
		// Last resort: wrap single object in array
		wrapped := []byte("[" + string(raw) + "]")
		if err2 := json.Unmarshal(wrapped, result); err2 != nil {
			return fmt.Errorf("parsing parliament response: %w", err)
		}
	}
	return nil
}

// ParlMetadata fetches the $metadata XML document via curl.
func (c *Client) ParlMetadata() ([]byte, error) {
	return c.parlCurl(parlBaseURL + "$metadata")
}

// metadataSchema is a minimal XML structure for parsing OData $metadata.
// The Parliament API uses edmx:Edmx with Microsoft namespaces.
type metadataSchema struct {
	XMLName      xml.Name        `xml:"Edmx"`
	DataServices metadataDataSvc `xml:"DataServices"`
}

type metadataDataSvc struct {
	Schemas []metadataSchemaNode `xml:"Schema"`
}

type metadataSchemaNode struct {
	Namespace      string                    `xml:"Namespace,attr"`
	EntityTypes    []metadataEntityType      `xml:"EntityType"`
	EntityContainers []metadataEntityContainer `xml:"EntityContainer"`
}

type metadataEntityContainer struct {
	EntitySets []metadataEntitySet `xml:"EntitySet"`
}

type metadataEntityType struct {
	Name       string              `xml:"Name,attr"`
	Properties []metadataProperty  `xml:"Property"`
}

type metadataProperty struct {
	Name     string `xml:"Name,attr"`
	Type     string `xml:"Type,attr"`
	Nullable string `xml:"Nullable,attr"`
}

type metadataEntitySet struct {
	Name       string `xml:"Name,attr"`
	EntityType string `xml:"EntityType,attr"`
}

// ParlTables returns the list of entity set names from $metadata.
func (c *Client) ParlTables() ([]string, error) {
	data, err := c.ParlMetadata()
	if err != nil {
		return nil, err
	}

	var schema metadataSchema
	if err := xml.Unmarshal(data, &schema); err != nil {
		// Fall back to regex extraction if XML parsing fails.
		return parlTablesFromRegex(data), nil
	}

	var tables []string
	for _, s := range schema.DataServices.Schemas {
		for _, ec := range s.EntityContainers {
			for _, es := range ec.EntitySets {
				tables = append(tables, es.Name)
			}
		}
	}
	if len(tables) == 0 {
		return parlTablesFromRegex(data), nil
	}
	return tables, nil
}

func parlTablesFromRegex(data []byte) []string {
	re := regexp.MustCompile(`<EntitySet\s+Name="([^"]+)"`)
	matches := re.FindAllSubmatch(data, -1)
	var tables []string
	for _, m := range matches {
		tables = append(tables, string(m[1]))
	}
	return tables
}

// ParlSchema returns the properties (columns) for a given entity type name.
func (c *Client) ParlSchema(table string) ([]metadataProperty, error) {
	data, err := c.ParlMetadata()
	if err != nil {
		return nil, err
	}

	var schema metadataSchema
	if err := xml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parsing metadata XML: %w", err)
	}

	// Find entity type matching the table name. The EntitySet name typically
	// matches the EntityType name, so we try both exact and trimmed matches.
	for _, s := range schema.DataServices.Schemas {
		for _, et := range s.EntityTypes {
			if strings.EqualFold(et.Name, table) {
				return et.Properties, nil
			}
		}
	}
	return nil, fmt.Errorf("entity type %q not found in metadata", table)
}

// ParsePeriod converts a human-friendly period string to a start time.
// Supported: "today", "week", "month", "last N days", or "YYYY-MM-DD".
func ParsePeriod(period string) (time.Time, error) {
	now := time.Now()
	p := strings.ToLower(strings.TrimSpace(period))

	switch p {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "week", "this week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location()), nil
	case "month", "this month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), nil
	}

	// "last N days"
	if strings.HasPrefix(p, "last ") && strings.HasSuffix(p, " days") {
		nStr := strings.TrimSuffix(strings.TrimPrefix(p, "last "), " days")
		n, err := strconv.Atoi(nStr)
		if err == nil && n > 0 {
			return now.AddDate(0, 0, -n), nil
		}
	}

	// Try YYYY-MM-DD
	t, err := time.Parse("2006-01-02", p)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unknown period %q (use: today, week, month, \"last N days\", or YYYY-MM-DD)", period)
}

// ODataDateTimeFilter formats a time for OData v3 datetime filter.
func ODataDateTimeFilter(field string, op string, t time.Time) string {
	return fmt.Sprintf("%s %s datetime'%s'", field, op, t.Format("2006-01-02T15:04:05"))
}

// ParseODataDate converts an OData v3 date string like "/Date(1234567890000)/"
// to a "2006-01-02" formatted string. Returns the original string if parsing fails.
func ParseODataDate(s string) string {
	if s == "" {
		return ""
	}
	re := regexp.MustCompile(`/Date\((-?\d+)\)/`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return s
	}
	ms, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return s
	}
	t := time.Unix(0, ms*int64(time.Millisecond))
	return t.Format("2006-01-02")
}
