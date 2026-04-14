package api

// Swissreg's /database/resources/query/search endpoint returns a Solr-flavored
// response: each result is a map of field -> []string (all values are arrays,
// even when single-valued). Field names carry type suffixes like
// `__type_text`, `__type_i18n`, `__type_date`.
type SwissregSearchResponse struct {
	Results    []SwissregResult `json:"results"`
	TotalItems int              `json:"totalItems"`
	PageSize   int              `json:"pageSize"`
	Target     string           `json:"target"`
	QueryText  string           `json:"queryText"`
}

type SwissregResult map[string][]string

// First returns the first value for a field, or "" if absent.
func (r SwissregResult) First(key string) string {
	if vs, ok := r[key]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}
