package api

import (
	"encoding/json"

	"github.com/matthiasak/chli/output"
)

// CKANResponse is the top-level envelope returned by all CKAN API actions.
type CKANResponse struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
}

// CKANSearchResult is the "result" object from package_search.
type CKANSearchResult struct {
	Count   int           `json:"count"`
	Results []CKANDataset `json:"results"`
}

// CKANDataset represents a single dataset (package) on opendata.swiss.
type CKANDataset struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Title            output.MultilingualText  `json:"title"`
	Description      output.MultilingualText  `json:"description"`
	Organization     CKANOrg                  `json:"organization"`
	Resources        []CKANResource           `json:"resources"`
	NumResources     int                      `json:"num_resources"`
	Issued           string                   `json:"issued"`
	MetadataModified string                   `json:"metadata_modified"`
	Keywords         map[string][]string      `json:"keywords"`
}

// CKANResource represents a single resource (file/API) within a dataset.
type CKANResource struct {
	ID          string                  `json:"id"`
	Name        output.MultilingualText `json:"name"`
	Format      string                  `json:"format"`
	DownloadURL string                  `json:"download_url"`
	MediaType   string                  `json:"media_type"`
}

// CKANOrg represents an organization on opendata.swiss.
type CKANOrg struct {
	Name        string                  `json:"name"`
	Title       output.MultilingualText `json:"title"`
	Description output.MultilingualText `json:"description"`
}
