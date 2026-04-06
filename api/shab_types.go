package api

import (
	"encoding/xml"

	"github.com/matthiasak/chli/output"
)

// SHABPublicationXML represents the parsed XML of a single SHAB publication.
type SHABPublicationXML struct {
	XMLName xml.Name       `xml:"publication" json:"-"`
	Meta    SHABXMLMeta    `xml:"meta" json:"meta"`
	Content SHABXMLContent `xml:"content" json:"content"`
}

// SHABXMLMeta holds metadata fields from the publication XML.
type SHABXMLMeta struct {
	PublicationNumber  string `xml:"publicationNumber" json:"publicationNumber"`
	PublicationDate    string `xml:"publicationDate" json:"publicationDate"`
	Rubric             string `xml:"rubric" json:"rubric"`
	SubRubric          string `xml:"subRubric" json:"subRubric,omitempty"`
	Language           string `xml:"language" json:"language,omitempty"`
	RegistrationOffice string `xml:"registrationOffice" json:"registrationOffice,omitempty"`
}

// SHABXMLContent wraps the inner shabContent element.
type SHABXMLContent struct {
	SHABContent SHABXMLSHABContent `xml:"shabContent" json:"shabContent"`
}

// SHABXMLSHABContent holds the core publication content.
type SHABXMLSHABContent struct {
	PublicationText SHABXMLText `xml:"publicationText" json:"publicationText"`
	Message         string      `xml:"message" json:"message,omitempty"`
}

// SHABXMLText holds multilingual publication text.
type SHABXMLText struct {
	DE string `xml:"de" json:"de,omitempty"`
	FR string `xml:"fr" json:"fr,omitempty"`
	IT string `xml:"it" json:"it,omitempty"`
}

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
