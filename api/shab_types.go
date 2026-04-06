package api

import "github.com/matthiasak/chli/output"

// SHABSearchResult is the top-level response from the SHAB publications search.
type SHABSearchResult struct {
	Content     []SHABPublication `json:"content"`
	Total       int               `json:"total"`
	PageRequest SHABPageRequest   `json:"pageRequest"`
}

// SHABPublication represents a single publication entry.
type SHABPublication struct {
	Meta SHABMeta `json:"meta"`
}

// SHABMeta holds metadata for a SHAB publication.
type SHABMeta struct {
	ID                 string                   `json:"id"`
	Rubric             string                   `json:"rubric"`
	SubRubric          string                   `json:"subRubric"`
	Language           string                   `json:"language"`
	PublicationNumber  string                   `json:"publicationNumber"`
	PublicationState   string                   `json:"publicationState"`
	PublicationDate    string                   `json:"publicationDate"`
	ExpirationDate     string                   `json:"expirationDate"`
	Title              output.MultilingualText  `json:"title"`
	Cantons            []string                 `json:"cantons"`
	RegistrationOffice *SHABOffice              `json:"registrationOffice,omitempty"`
}

// SHABOffice represents a registration office.
type SHABOffice struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	Street       string `json:"street,omitempty"`
	Town         string `json:"town,omitempty"`
	SwissZipCode string `json:"swissZipCode,omitempty"`
}

// SHABPageRequest holds pagination info from the response.
type SHABPageRequest struct {
	Page int `json:"page"`
	Size int `json:"size"`
}
