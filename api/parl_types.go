package api

import "encoding/json"

// ODataResponse is the wrapper for OData v3 JSON responses.
// The "d" field can be either an array or a single object.
type ODataResponse struct {
	D json.RawMessage `json:"d"`
}

// ParlPerson represents a person record from the Parliament API.
// Many fields are nullable in the OData API, so we use *int / *string for those.
type ParlPerson struct {
	ID                 int     `json:"ID"`
	Language           string  `json:"Language"`
	PersonNumber       int     `json:"PersonNumber"`
	PersonIdCode       *int    `json:"PersonIdCode"`
	Title              *string `json:"Title"`
	TitleText          *string `json:"TitleText"`
	LastName           string  `json:"LastName"`
	FirstName          string  `json:"FirstName"`
	OfficialName       string  `json:"OfficialName"`
	GenderAsString     string  `json:"GenderAsString"`
	DateOfBirth        *string `json:"DateOfBirth"`
	DateOfDeath        *string `json:"DateOfDeath"`
	MaritalStatus      *int    `json:"MaritalStatus"`
	PlaceOfBirthCity   *string `json:"PlaceOfBirthCity"`
	PlaceOfBirthCanton *string `json:"PlaceOfBirthCanton"`
	MilitaryRank       *int    `json:"MilitaryRank"`
	MilitaryRankText   *string `json:"MilitaryRankText"`
	NativeLanguage     *string `json:"NativeLanguage"`
	NumberOfChildren   *int    `json:"NumberOfChildren"`
	Modified           string  `json:"Modified"`
}

// ParlBusiness represents a parliamentary business (motion, interpellation, etc.).
type ParlBusiness struct {
	ID                                int     `json:"ID"`
	Language                          string  `json:"Language"`
	BusinessShortNumber               string  `json:"BusinessShortNumber"`
	BusinessType                      int     `json:"BusinessType"`
	BusinessTypeName                  string  `json:"BusinessTypeName"`
	BusinessTypeAbbreviation          string  `json:"BusinessTypeAbbreviation"`
	Title                             string  `json:"Title"`
	Description                       *string `json:"Description"`
	InitialSituation                  *string `json:"InitialSituation"`
	Proceedings                       *string `json:"Proceedings"`
	DraftText                         *string `json:"DraftText"`
	SubmittedText                     *string `json:"SubmittedText"`
	ReasonText                        *string `json:"ReasonText"`
	DocumentationText                 *string `json:"DocumentationText"`
	MotionText                        *string `json:"MotionText"`
	FederalCouncilResponseText        *string `json:"FederalCouncilResponseText"`
	FederalCouncilProposal            *int    `json:"FederalCouncilProposal"`
	FederalCouncilProposalText        *string `json:"FederalCouncilProposalText"`
	FederalCouncilProposalDate        *string `json:"FederalCouncilProposalDate"`
	SubmittedBy                       *string `json:"SubmittedBy"`
	BusinessStatus                    int     `json:"BusinessStatus"`
	BusinessStatusText                string  `json:"BusinessStatusText"`
	BusinessStatusDate                *string `json:"BusinessStatusDate"`
	ResponsibleDepartmentName         *string `json:"ResponsibleDepartmentName"`
	ResponsibleDepartmentAbbreviation *string `json:"ResponsibleDepartmentAbbreviation"`
	SubmissionDate                    *string `json:"SubmissionDate"`
	SubmissionCouncilName             *string `json:"SubmissionCouncilName"`
	Tags                              *string `json:"Tags"`
	TagNames                          *string `json:"TagNames"`
	Category                          *string `json:"Category"`
	Modified                          string  `json:"Modified"`
}

// ParlVote represents a vote (ballot) on a business item.
type ParlVote struct {
	ID                  int     `json:"ID"`
	Language            string  `json:"Language"`
	RegistrationNumber  *int    `json:"RegistrationNumber"`
	BusinessNumber      *int    `json:"BusinessNumber"`
	BusinessShortNumber *string `json:"BusinessShortNumber"`
	BusinessTitle       *string `json:"BusinessTitle"`
	BusinessAuthor      *string `json:"BusinessAuthor"`
	BillNumber          *int    `json:"BillNumber"`
	BillTitle           *string `json:"BillTitle"`
	IdSession           *int    `json:"IdSession"`
	SessionName         *string `json:"SessionName"`
	Subject             *string `json:"Subject"`
	MeaningYes          *string `json:"MeaningYes"`
	MeaningNo           *string `json:"MeaningNo"`
	VoteEnd             *string `json:"VoteEnd"`
}

// ParlVoting represents an individual councillor's vote.
type ParlVoting struct {
	ID            int     `json:"ID"`
	Language      string  `json:"Language"`
	IdVote        *int    `json:"IdVote"`
	PersonNumber  int     `json:"PersonNumber"`
	FirstName     string  `json:"FirstName"`
	LastName      string  `json:"LastName"`
	Canton        *string `json:"Canton"`
	ParlGroupName *string `json:"ParlGroupName"`
	Decision      *int    `json:"Decision"`
	DecisionText  *string `json:"DecisionText"`
	Modified      string  `json:"Modified"`
}

// ParlSession represents a parliamentary session.
type ParlSession struct {
	ID           int     `json:"ID"`
	Language     string  `json:"Language"`
	Code         *string `json:"Code"`
	Title        *string `json:"Title"`
	Abbreviation *string `json:"Abbreviation"`
	StartDate    *string `json:"StartDate"`
	EndDate      *string `json:"EndDate"`
	Type         *int    `json:"Type"`
	Modified     string  `json:"Modified"`
}

// ParlCommittee represents a parliamentary committee.
type ParlCommittee struct {
	ID              int     `json:"ID"`
	Language        string  `json:"Language"`
	CommitteeNumber *int    `json:"CommitteeNumber"`
	Abbreviation    *string `json:"Abbreviation"`
	Name            *string `json:"Name"`
	Council         *int    `json:"Council"`
	CouncilName     *string `json:"CouncilName"`
	TypeCode        *int    `json:"TypeCode"`
	Modified        string  `json:"Modified"`
}

// ParlParty represents a political party.
type ParlParty struct {
	ID           int     `json:"ID"`
	Language     string  `json:"Language"`
	PartyNumber  *int    `json:"PartyNumber"`
	PartyName    *string `json:"PartyName"`
	Abbreviation *string `json:"Abbreviation"`
	StartDate    *string `json:"StartDate"`
	EndDate      *string `json:"EndDate"`
	Modified     string  `json:"Modified"`
}

// ParlCanton represents a Swiss canton.
type ParlCanton struct {
	ID           int     `json:"ID"`
	Language     string  `json:"Language"`
	CantonNumber *int    `json:"CantonNumber"`
	CantonName   *string `json:"CantonName"`
	Abbreviation *string `json:"CantonAbbreviation"`
	Modified     string  `json:"Modified"`
}

// ParlMemberCouncil represents a council membership record.
type ParlMemberCouncil struct {
	ID                int     `json:"ID"`
	Language          string  `json:"Language"`
	PersonNumber      int     `json:"PersonNumber"`
	PersonIdCode      *int    `json:"PersonIdCode"`
	FirstName         string  `json:"FirstName"`
	LastName          string  `json:"LastName"`
	GenderAsString    string  `json:"GenderAsString"`
	Party             *string `json:"PartyName"`
	PartyAbbreviation *string `json:"PartyAbbreviation"`
	Council           *int    `json:"Council"`
	CouncilName       *string `json:"CouncilName"`
	Canton            *int    `json:"Canton"`
	CantonName        *string `json:"CantonName"`
	CantonAbbreviation *string `json:"CantonAbbreviation"`
	Active            bool    `json:"Active"`
	DateJoining       *string `json:"DateJoining"`
	DateLeaving       *string `json:"DateLeaving"`
	Modified          string  `json:"Modified"`
}

// ParlPersonDetail holds extended person info from the Person entity.
type ParlPersonDetail struct {
	ID                 int     `json:"ID"`
	Language           string  `json:"Language"`
	PersonNumber       int     `json:"PersonNumber"`
	PersonIdCode       *int    `json:"PersonIdCode"`
	Title              *int    `json:"Title"`
	TitleText          *string `json:"TitleText"`
	LastName           string  `json:"LastName"`
	FirstName          string  `json:"FirstName"`
	OfficialName       string  `json:"OfficialName"`
	GenderAsString     string  `json:"GenderAsString"`
	DateOfBirth        *string `json:"DateOfBirth"`
	DateOfDeath        *string `json:"DateOfDeath"`
	MaritalStatus      *int    `json:"MaritalStatus"`
	MaritalStatusText  *string `json:"MaritalStatusText"`
	PlaceOfBirthCity   *string `json:"PlaceOfBirthCity"`
	PlaceOfBirthCanton *string `json:"PlaceOfBirthCanton"`
	MilitaryRank       *int    `json:"MilitaryRank"`
	MilitaryRankText   *string `json:"MilitaryRankText"`
	NativeLanguage     *string `json:"NativeLanguage"`
	NumberOfChildren   *int    `json:"NumberOfChildren"`
	Modified           string  `json:"Modified"`
}

// ParlPersonCommunication holds contact info (email, homepage, phone).
type ParlPersonCommunication struct {
	ID                    string  `json:"ID"`
	Language              string  `json:"Language"`
	PersonNumber          int     `json:"PersonNumber"`
	Address               *string `json:"Address"`
	CommunicationType     *int    `json:"CommunicationType"`
	CommunicationTypeText *string `json:"CommunicationTypeText"`
}

// ParlPersonInterest represents a declared interest from the Parliament API.
type ParlPersonInterest struct {
	ID                       string  `json:"ID"`
	Language                 string  `json:"Language"`
	PersonNumber             int     `json:"PersonNumber"`
	InterestName             *string `json:"InterestName"`
	InterestType             *int    `json:"InterestType"`
	InterestTypeText         *string `json:"InterestTypeText"`
	InterestTypeShortText    *string `json:"InterestTypeShortText"`
	FunctionInAgency         *int    `json:"FunctionInAgency"`
	FunctionInAgencyText     *string `json:"FunctionInAgencyText"`
	FunctionInAgencyShortText *string `json:"FunctionInAgencyShortText"`
	OrganizationType         *int    `json:"OrganizationType"`
	OrganizationTypeText     *string `json:"OrganizationTypeText"`
	Paid                     *bool   `json:"Paid"`
	SortOrder                *int    `json:"SortOrder"`
	Modified                 string  `json:"Modified"`
}

// ParlBusinessStatusEntry represents a status change in a business item's timeline.
type ParlBusinessStatusEntry struct {
	ID                 string  `json:"ID"`
	Language           string  `json:"Language"`
	BusinessNumber     *int    `json:"BusinessNumber"`
	BusinessStatusId   *int    `json:"BusinessStatusId"`
	BusinessStatusName *string `json:"BusinessStatusName"`
	BusinessStatusDate *string `json:"BusinessStatusDate"`
	IsMotionInSecondCouncil *bool `json:"IsMotionInSecondCouncil"`
	Modified           string  `json:"Modified"`
}

// ParlPreconsultation represents a committee assignment for a business item.
type ParlPreconsultation struct {
	ID                  string  `json:"ID"`
	Language            string  `json:"Language"`
	BusinessNumber      *int    `json:"BusinessNumber"`
	BusinessShortNumber *string `json:"BusinessShortNumber"`
	CommitteeNumber     *int    `json:"CommitteeNumber"`
	CommitteeName       *string `json:"CommitteeName"`
	Abbreviation1       *string `json:"Abbreviation1"`
	PreconsultationDate *string `json:"PreconsultationDate"`
	TreatmentCategory   *string `json:"TreatmentCategory"`
	Modified            string  `json:"Modified"`
}

// ParlPublication represents a document linked to a business item.
type ParlPublication struct {
	ID                      string  `json:"ID"`
	Language                string  `json:"Language"`
	PublicationType         *int    `json:"PublicationType"`
	PublicationTypeName     *string `json:"PublicationTypeName"`
	PublicationTypeAbbreviation *string `json:"PublicationTypeAbbreviation"`
	Title                   *string `json:"Title"`
	Page                    *int    `json:"Page"`
	Volume                  *string `json:"Volume"`
	Year                    *int    `json:"Year"`
	BusinessNumber          *int    `json:"BusinessNumber"`
	BusinessShortNumber     *string `json:"BusinessShortNumber"`
	Modified                string  `json:"Modified"`
}

// ParlBusinessRole represents a person's role in a parliamentary business.
type ParlBusinessRole struct {
	ID                     string  `json:"ID"`
	Language               string  `json:"Language"`
	Role                   *int    `json:"Role"`
	RoleName               *string `json:"RoleName"`
	BusinessNumber         *int    `json:"BusinessNumber"`
	MemberCouncilNumber    *int    `json:"MemberCouncilNumber"`
	BusinessShortNumber    *string `json:"BusinessShortNumber"`
	BusinessTitle          *string `json:"BusinessTitle"`
	BusinessSubmissionDate *string `json:"BusinessSubmissionDate"`
	BusinessType           *int    `json:"BusinessType"`
	BusinessTypeName       *string `json:"BusinessTypeName"`
	BusinessTypeAbbreviation *string `json:"BusinessTypeAbbreviation"`
	Modified               string  `json:"Modified"`
}

// ParlMemberCommittee represents a person's committee membership.
type ParlMemberCommittee struct {
	ID                    string  `json:"ID"`
	Language              string  `json:"Language"`
	CommitteeNumber       *int    `json:"CommitteeNumber"`
	PersonNumber          int     `json:"PersonNumber"`
	CommitteeFunction     *int    `json:"CommitteeFunction"`
	CommitteeFunctionName *string `json:"CommitteeFunctionName"`
	CommitteeName         *string `json:"CommitteeName"`
	Abbreviation1         *string `json:"Abbreviation1"`
	CommitteeType         *int    `json:"CommitteeType"`
	CommitteeTypeName     *string `json:"CommitteeTypeName"`
	CouncilName           *string `json:"CouncilName"`
	Modified              string  `json:"Modified"`
}

// ParlSubjectBusiness links a business to a debate subject (for transcripts).
type ParlSubjectBusiness struct {
	IdSubject           string  `json:"IdSubject"`
	BusinessNumber      int     `json:"BusinessNumber"`
	Language            string  `json:"Language"`
	BusinessShortNumber string  `json:"BusinessShortNumber"`
	Title               *string `json:"Title"`
	TitleDE             *string `json:"TitleDE"`
	Modified            string  `json:"Modified"`
}

// ParlTranscript represents a parliamentary debate transcript entry.
type ParlTranscript struct {
	ID           string  `json:"ID"`
	Language     string  `json:"Language"`
	IdSubject    string  `json:"IdSubject"`
	PersonNumber *int    `json:"PersonNumber"`
	Type         *int    `json:"Type"`
	Text         *string `json:"Text"`
	Start        *string `json:"Start"`
	End          *string `json:"End"`
	Function     *string `json:"Function"`
	DisplayName  *string `json:"DisplayName"`
	Modified     string  `json:"Modified"`
}

// Str safely dereferences a *string, returning "" if nil.
func Str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Int safely dereferences a *int, returning 0 if nil.
func Int(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
