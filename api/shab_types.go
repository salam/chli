package api

import (
	"encoding/xml"

	"github.com/matthiasak/chli/output"
)

// SHABPublicationXML is the parsed XML of a single SHAB publication.
type SHABPublicationXML struct {
	XMLName xml.Name       `xml:"publication" json:"-"`
	Meta    SHABXMLMeta    `xml:"meta" json:"meta"`
	Content SHABXMLContent `xml:"content" json:"content"`
}

// SHABXMLMeta holds metadata fields from the publication XML.
type SHABXMLMeta struct {
	ID                 string         `xml:"id" json:"id,omitempty"`
	PublicationNumber  string         `xml:"publicationNumber" json:"publicationNumber"`
	PublicationState   string         `xml:"publicationState" json:"publicationState,omitempty"`
	PublicationDate    string         `xml:"publicationDate" json:"publicationDate"`
	Rubric             string         `xml:"rubric" json:"rubric"`
	SubRubric          string         `xml:"subRubric" json:"subRubric,omitempty"`
	Language           string         `xml:"language" json:"language,omitempty"`
	Cantons            string         `xml:"cantons" json:"cantons,omitempty"`
	LegalRemedy        string         `xml:"legalRemedy" json:"legalRemedy,omitempty"`
	Title              *SHABXMLTitle  `xml:"title" json:"title,omitempty"`
	RegistrationOffice *SHABXMLOffice `xml:"registrationOffice" json:"registrationOffice,omitempty"`
}

// SHABXMLTitle holds the multilingual <title> block.
type SHABXMLTitle struct {
	DE string `xml:"de" json:"de,omitempty"`
	FR string `xml:"fr" json:"fr,omitempty"`
	IT string `xml:"it" json:"it,omitempty"`
	EN string `xml:"en" json:"en,omitempty"`
}

// Pick returns the title in the preferred language, falling back to any available.
func (t *SHABXMLTitle) Pick(lang string) string {
	if t == nil {
		return ""
	}
	switch lang {
	case "fr":
		if t.FR != "" {
			return t.FR
		}
	case "it":
		if t.IT != "" {
			return t.IT
		}
	case "en":
		if t.EN != "" {
			return t.EN
		}
	}
	if t.DE != "" {
		return t.DE
	}
	if t.FR != "" {
		return t.FR
	}
	if t.IT != "" {
		return t.IT
	}
	return t.EN
}

// SHABXMLOffice is the structured <registrationOffice> block.
type SHABXMLOffice struct {
	ID           string `xml:"id" json:"id,omitempty"`
	DisplayName  string `xml:"displayName" json:"displayName,omitempty"`
	Street       string `xml:"street" json:"street,omitempty"`
	StreetNumber string `xml:"streetNumber" json:"streetNumber,omitempty"`
	SwissZipCode string `xml:"swissZipCode" json:"swissZipCode,omitempty"`
	Town         string `xml:"town" json:"town,omitempty"`
}

// SHABXMLContent is the <content> body. For HR publications it contains the
// company commons, transaction, and lastFosc pointer directly (no shabContent
// wrapper despite what older schemas suggested).
type SHABXMLContent struct {
	PublicationText SHABXMLText      `xml:"publicationText" json:"publicationText,omitempty"`
	Message         string           `xml:"message" json:"message,omitempty"`
	JournalNumber   string           `xml:"journalNumber" json:"journalNumber,omitempty"`
	JournalDate     string           `xml:"journalDate" json:"journalDate,omitempty"`
	CommonsNew      *SHABCommons     `xml:"commonsNew" json:"commonsNew,omitempty"`
	CommonsActual   *SHABCommons     `xml:"commonsActual" json:"commonsActual,omitempty"`
	LastFosc        *SHABLastFosc    `xml:"lastFosc" json:"lastFosc,omitempty"`
	Transaction     *SHABTransaction `xml:"transaction" json:"transaction,omitempty"`
}

// SHABXMLText carries either multilingual children (<de>/<fr>/<it>) or a plain
// text body (HR publications put the text directly under <publicationText>).
type SHABXMLText struct {
	Body string `xml:",chardata" json:"body,omitempty"`
	DE   string `xml:"de" json:"de,omitempty"`
	FR   string `xml:"fr" json:"fr,omitempty"`
	IT   string `xml:"it" json:"it,omitempty"`
}

// PickText returns the text body in the preferred language, preferring the
// language children when present and falling back to the plain body.
func (t SHABXMLText) PickText(lang string) string {
	switch lang {
	case "fr":
		if t.FR != "" {
			return t.FR
		}
	case "it":
		if t.IT != "" {
			return t.IT
		}
	}
	if t.DE != "" {
		return t.DE
	}
	if t.FR != "" {
		return t.FR
	}
	if t.IT != "" {
		return t.IT
	}
	return t.Body
}

// SHABCommons is the shared structure of commonsNew / commonsActual.
type SHABCommons struct {
	Company  *SHABCompany  `xml:"company" json:"company,omitempty"`
	Purpose  string        `xml:"purpose" json:"purpose,omitempty"`
	Revision *SHABRevision `xml:"revision" json:"revision,omitempty"`
}

// SHABCompany represents the company block inside commons*.
type SHABCompany struct {
	Name      string       `xml:"name" json:"name"`
	UID       string       `xml:"uid" json:"uid,omitempty"`
	Code13    string       `xml:"code13" json:"code13,omitempty"`
	Seat      string       `xml:"seat" json:"seat,omitempty"`
	LegalForm string       `xml:"legalForm" json:"legalForm,omitempty"`
	Address   *SHABAddress `xml:"address" json:"address,omitempty"`
}

// SHABAddress is the street address inside a company block.
type SHABAddress struct {
	Street       string `xml:"street" json:"street,omitempty"`
	HouseNumber  string `xml:"houseNumber" json:"houseNumber,omitempty"`
	SwissZipCode string `xml:"swissZipCode" json:"swissZipCode,omitempty"`
	Town         string `xml:"town" json:"town,omitempty"`
}

// SHABRevision is the revision (audit) block inside a commons.
type SHABRevision struct {
	OptingOut       bool                 `xml:"optingOut" json:"optingOut"`
	RevisionCompany *SHABRevisionCompany `xml:"revisionCompany" json:"revisionCompany,omitempty"`
}

// SHABRevisionCompany is the auditor company.
type SHABRevisionCompany struct {
	Name    string `xml:"name" json:"name"`
	Country string `xml:"country" json:"country,omitempty"`
	UID     string `xml:"uid" json:"uid,omitempty"`
}

// SHABLastFosc is the back-pointer to the previous FOSC publication.
type SHABLastFosc struct {
	Date     string `xml:"lastFoscDate" json:"date,omitempty"`
	Number   string `xml:"lastFoscNumber" json:"number,omitempty"`
	Sequence string `xml:"lastFoscSequence" json:"sequence,omitempty"`
}

// SHABTransaction describes the transaction kind recorded in this publication.
type SHABTransaction struct {
	Update   *SHABTxUpdate `xml:"update" json:"update,omitempty"`
	Creation *struct{}     `xml:"creation" json:"creation,omitempty"`
	Deletion *struct{}     `xml:"deletion" json:"deletion,omitempty"`
}

// SHABTxUpdate is the transaction body for mutations.
type SHABTxUpdate struct {
	Changements SHABChangements `xml:"changements" json:"changements"`
}

// SHABChangements lists the top-level changed flags in an update transaction.
type SHABChangements struct {
	Others             bool `xml:"others" json:"others"`
	NameChanged        bool `xml:"nameChanged" json:"nameChanged"`
	UIDChanged         bool `xml:"uidChanged" json:"uidChanged"`
	LegalStatusChanged bool `xml:"legalStatusChanged" json:"legalStatusChanged"`
	SeatChanged        bool `xml:"seatChanged" json:"seatChanged"`
	AddressChanged     bool `xml:"addressChanged" json:"addressChanged"`
	PurposeChanged     bool `xml:"purposeChanged" json:"purposeChanged"`
}

// ChangedLabels returns human labels for the flags that are true, in a stable order.
func (c SHABChangements) ChangedLabels() []string {
	var out []string
	if c.NameChanged {
		out = append(out, "name")
	}
	if c.UIDChanged {
		out = append(out, "UID")
	}
	if c.LegalStatusChanged {
		out = append(out, "legal status")
	}
	if c.SeatChanged {
		out = append(out, "seat")
	}
	if c.AddressChanged {
		out = append(out, "address")
	}
	if c.PurposeChanged {
		out = append(out, "purpose")
	}
	if c.Others {
		out = append(out, "others")
	}
	return out
}

// SHABSearchResult is the top-level response from the SHAB publications search.
type SHABSearchResult struct {
	Content     []SHABPublication `json:"content"`
	Total       int               `json:"total"`
	PageRequest SHABPageRequest   `json:"pageRequest"`
}

// SHABPublication represents a single publication entry from search.
type SHABPublication struct {
	Meta SHABMeta `json:"meta"`
}

// SHABMeta holds metadata for a SHAB publication from search results.
type SHABMeta struct {
	ID                 string                  `json:"id"`
	Rubric             string                  `json:"rubric"`
	SubRubric          string                  `json:"subRubric"`
	Language           string                  `json:"language"`
	PublicationNumber  string                  `json:"publicationNumber"`
	PublicationState   string                  `json:"publicationState"`
	PublicationDate    string                  `json:"publicationDate"`
	ExpirationDate     string                  `json:"expirationDate"`
	Title              output.MultilingualText `json:"title"`
	Cantons            []string                `json:"cantons"`
	RegistrationOffice *SHABOffice             `json:"registrationOffice,omitempty"`
}

// SHABOffice represents a registration office in search results.
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
