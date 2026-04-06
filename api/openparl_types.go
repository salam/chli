package api

import "github.com/matthiasak/chli/output"

// OpenParlResponse wraps paginated responses from api.openparldata.ch.
type OpenParlResponse[T any] struct {
	Data []T          `json:"data"`
	Meta OpenParlMeta `json:"meta"`
}

type OpenParlMeta struct {
	Offset       int  `json:"offset"`
	Limit        int  `json:"limit"`
	TotalRecords int  `json:"total_records"`
	HasMore      bool `json:"has_more"`
}

// OpenParlPerson is a minimal person record from OpenParlData.
type OpenParlPerson struct {
	ID         int    `json:"id"`
	Fullname   string `json:"fullname"`
	ExternalID string `json:"external_id"`
	BodyKey    string `json:"body_key"`
}

// OpenParlInterest represents a declared interest / corporate role.
type OpenParlInterest struct {
	ID                 int                  `json:"id"`
	PersonID           int                  `json:"person_id"`
	PersonExternalID   string               `json:"person_external_id,omitempty"`
	PersonFullname     string               `json:"person_fullname,omitempty"` // populated by us after resolving
	Name               output.MultilingualText `json:"name"`
	NameShort          output.MultilingualText `json:"name_short"`
	RoleName           output.MultilingualText `json:"role_name"`
	Group              output.MultilingualText `json:"group"`
	TypePayment        output.MultilingualText `json:"type_payment"`
	TypePaymentHarmonized *string             `json:"type_payment_harmonized"`
	URL                *string              `json:"url"`
	BeginDate          *string              `json:"begin_date"`
	EndDate            *string              `json:"end_date"`
	ExOfficio          *bool                `json:"ex_officio"`
}

// OpenParlAccessBadge represents a lobby access badge.
type OpenParlAccessBadge struct {
	ID                      int                     `json:"id"`
	PersonID                int                     `json:"person_id"`
	PersonExternalID        string                  `json:"person_external_id"`
	PersonFullname          string                  `json:"person_fullname"`
	BeneficiaryPersonID     *int                    `json:"beneficiary_person_id"`
	BeneficiaryPersonFullname *string               `json:"beneficiary_person_fullname"`
	BeneficiaryGroup        *string                 `json:"beneficiary_group"`
	TypeHarmonized          *string                 `json:"type_harmonized"`
	ValidFrom               *string                 `json:"valid_from"`
	ValidTo                 *string                 `json:"valid_to"`
	Type                    output.MultilingualText `json:"type"`
}
