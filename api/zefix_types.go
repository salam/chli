package api

// ZefixCompany is the common shape returned by Zefix endpoints (search +
// single-company lookups). Only fields the CLI surfaces are typed; unknown
// fields are preserved via json.RawMessage would add weight, so we accept
// partial decoding.
type ZefixCompany struct {
	EhraID          int64            `json:"ehraid"`
	CHID            string           `json:"chid"`
	UID             string           `json:"uid"`
	UIDFormatted    string           `json:"uidFormatted"`
	LegalSeat       string           `json:"legalSeat"`
	LegalSeatID     int              `json:"legalSeatId"`
	Name            string           `json:"name"`
	Canton          string           `json:"canton"`
	Status          string           `json:"status"`
	StatusCode      int              `json:"statusCode"`
	LegalForm       *ZefixLegalForm  `json:"legalForm,omitempty"`
	Address         *ZefixAddress    `json:"address,omitempty"`
	SogcDate        string           `json:"sogcDate,omitempty"`
	DeletionDate    string           `json:"deletionDate,omitempty"`
	RegisterOffice  *ZefixRegOffice  `json:"registerOffice,omitempty"`
	Purpose         string           `json:"purpose,omitempty"`
	HasTakenOver    bool             `json:"hasTakenOver,omitempty"`
	WasTakenOver    bool             `json:"wasTakenOver,omitempty"`
	OldNames        []ZefixOldName   `json:"oldNames,omitempty"`
	Publications    []ZefixPublication `json:"shabPub,omitempty"`
}

type ZefixLegalForm struct {
	ID             int              `json:"id"`
	Name           ZefixI18n        `json:"name"`
	ShortName      ZefixI18n        `json:"shortName"`
}

type ZefixI18n struct {
	De string `json:"de"`
	Fr string `json:"fr"`
	It string `json:"it"`
	En string `json:"en"`
}

func (t ZefixI18n) Pick(lang string) string {
	switch lang {
	case "fr":
		if t.Fr != "" {
			return t.Fr
		}
	case "it":
		if t.It != "" {
			return t.It
		}
	case "en":
		if t.En != "" {
			return t.En
		}
	}
	return t.De
}

type ZefixAddress struct {
	Organisation string `json:"organisation,omitempty"`
	CareOf       string `json:"careOf,omitempty"`
	Street       string `json:"street,omitempty"`
	HouseNumber  string `json:"houseNumber,omitempty"`
	AddressLine1 string `json:"addressLine1,omitempty"`
	AddressLine2 string `json:"addressLine2,omitempty"`
	SwissZipCode int    `json:"swissZipCode,omitempty"`
	City         string `json:"city,omitempty"`
	Country      string `json:"country,omitempty"`
}

type ZefixRegOffice struct {
	ID           int    `json:"id"`
	CantonalExcerptWeb string `json:"cantonalExcerptWeb,omitempty"`
	Canton       string `json:"canton,omitempty"`
}

type ZefixOldName struct {
	Name     string `json:"name"`
	DateFrom string `json:"dateFrom,omitempty"`
	DateTo   string `json:"dateTo,omitempty"`
}

type ZefixPublication struct {
	ShabDate    string `json:"shabDate,omitempty"`
	ShabID      int64  `json:"shabId,omitempty"`
	ShabNr      int64  `json:"shabNr,omitempty"`
	ShabSubRub  string `json:"shabSubRubrik,omitempty"`
	Mutation    string `json:"mutationTypes,omitempty"`
	Message     string `json:"message,omitempty"`
}

type ZefixSearchRequest struct {
	Name             string   `json:"name,omitempty"`
	Canton           string   `json:"canton,omitempty"`
	LegalForm        []int    `json:"legalForm,omitempty"`
	LanguageKey      string   `json:"languageKey,omitempty"`
	DeletedFilter    string   `json:"deletedFilter,omitempty"`
	MaxEntries       int      `json:"maxEntries,omitempty"`
}
