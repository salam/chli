package api

// Plate-related data structures. The plate vertical never touches personal
// data — these types describe the cantonal process only.

// Plate is the parsed form of a user-supplied plate string.
type Plate struct {
	// Raw is the original user input, trimmed only.
	Raw string `json:"raw"`
	// Normalized is the plate with all whitespace/hyphens removed, canton
	// prefix uppercased. E.g. "ZH 123 456" -> "ZH123456".
	Normalized string `json:"normalized"`
	// Canton is the detected two-letter code (e.g. "ZH") or "" in fulltext mode.
	Canton string `json:"canton,omitempty"`
	// Digits is the numeric tail after the canton prefix (e.g. "123456").
	// May be empty in fulltext mode or non-numeric for special plates.
	Digits string `json:"digits,omitempty"`
	// Warnings are non-fatal parser notes, e.g. non-digit body.
	Warnings []string `json:"warnings,omitempty"`
}

// HalterauskunftMode is the service style a canton offers.
type HalterauskunftMode string

const (
	ModeOnline      HalterauskunftMode = "online"
	ModePostal      HalterauskunftMode = "postal"
	ModeMixed       HalterauskunftMode = "mixed"
	ModeUnavailable HalterauskunftMode = "unavailable"
)

// CaptchaKind describes the anti-bot challenge on the landing page.
type CaptchaKind string

const (
	CaptchaNone      CaptchaKind = "none"
	CaptchaHCaptcha  CaptchaKind = "hcaptcha"
	CaptchaReCaptcha CaptchaKind = "recaptcha"
)

// ProcessingTyp is the typical turnaround category.
type ProcessingTyp string

const (
	ProcInstant ProcessingTyp = "instant"
	ProcHours   ProcessingTyp = "hours"
	Proc1to3    ProcessingTyp = "1-3_days"
	Proc5to10   ProcessingTyp = "5-10_days"
)

// Delivery is how the answer is returned.
type Delivery string

const (
	DeliveryPDFEmail     Delivery = "pdf_email"
	DeliveryPostal       Delivery = "postal"
	DeliveryOnlinePortal Delivery = "online_portal"
)

// VerifiedBy is the provenance of the last verification.
type VerifiedBy string

const (
	VerifiedManual VerifiedBy = "manual"
	VerifiedCI     VerifiedBy = "ci"
)

// Postal is a street address.
type Postal struct {
	Street string `json:"street,omitempty"`
	Zip    string `json:"zip,omitempty"`
	City   string `json:"city,omitempty"`
}

// IsEmpty reports whether the postal address has no fields set.
func (p Postal) IsEmpty() bool {
	return p.Street == "" && p.Zip == "" && p.City == ""
}

// Authority is the cantonal Strassenverkehrsamt or equivalent.
type Authority struct {
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	Email  string `json:"email,omitempty"`
	Phone  string `json:"phone,omitempty"`
	Postal Postal `json:"postal,omitempty"`
}

// AuthReqs describes what the lookup form requires from the requester.
type AuthReqs struct {
	Captcha                CaptchaKind `json:"captcha"`
	SMS                    bool        `json:"sms"`
	EmailConfirmation      bool        `json:"email_confirmation"`
	RequiresStatedReason   bool        `json:"requires_stated_reason"`
	RequiresIdentification bool        `json:"requires_identification"`
}

// Processing bundles turnaround + delivery.
type Processing struct {
	Typical  ProcessingTyp `json:"typical"`
	Delivery Delivery      `json:"delivery"`
}

// HalterauskunftEntry describes a cantonal holder-lookup service.
type HalterauskunftEntry struct {
	Mode             HalterauskunftMode `json:"mode"`
	URL              string             `json:"url,omitempty"`
	DeeplinkTemplate string             `json:"deeplink_template,omitempty"`
	FormPDF          string             `json:"form_pdf,omitempty"`
	CostCHF          float64            `json:"cost_chf"`
	PaymentMethods   []string           `json:"payment_methods"`
	Auth             AuthReqs           `json:"auth"`
	Processing       Processing         `json:"processing"`
	LegalBasis       string             `json:"legal_basis,omitempty"`
	Notes            map[string]string  `json:"notes,omitempty"`
	// Queryable marks cantons offering free, self-serve public lookups that a
	// user can complete themselves in a browser (no CHF, no login). chli never
	// automates these — the captcha on every such service is the cantonal
	// gate, not a bug to be worked around. --query opens the lookup URL and
	// increments a local 3/24h quota.
	Queryable bool `json:"queryable,omitempty"`
}

// Verification is provenance metadata for the canton record.
type Verification struct {
	LastVerified string     `json:"last_verified"`
	VerifiedBy   VerifiedBy `json:"verified_by"`
	SourceURLs   []string   `json:"source_urls,omitempty"`
}

// CantonEntry is one canton's complete dispatcher record.
type CantonEntry struct {
	Code           string              `json:"code"`
	Names          map[string]string   `json:"names"`
	Authority      Authority           `json:"authority"`
	Halterauskunft HalterauskunftEntry `json:"halterauskunft"`
	Verification   Verification        `json:"verification"`
}

// Name returns the canton's display name in the preferred language,
// falling back to de, then en, then the code itself.
func (c CantonEntry) Name(lang string) string {
	if v, ok := c.Names[lang]; ok && v != "" {
		return v
	}
	if v, ok := c.Names["de"]; ok && v != "" {
		return v
	}
	if v, ok := c.Names["en"]; ok && v != "" {
		return v
	}
	return c.Code
}

// Note returns the halterauskunft note in the preferred language,
// falling back to de, then en, then "".
func (c CantonEntry) Note(lang string) string {
	n := c.Halterauskunft.Notes
	if n == nil {
		return ""
	}
	if v, ok := n[lang]; ok && v != "" {
		return v
	}
	if v, ok := n["de"]; ok && v != "" {
		return v
	}
	if v, ok := n["en"]; ok && v != "" {
		return v
	}
	return ""
}

// PlateCantonsFile is the on-disk shape of plate_cantons.json.
type PlateCantonsFile struct {
	SchemaVersion int                    `json:"schema_version"`
	Cantons       map[string]CantonEntry `json:"cantons"`
}

// VerifyStatus categorises a cantonal endpoint probe result.
type VerifyStatus string

const (
	VerifyOK    VerifyStatus = "ok"
	VerifyWarn  VerifyStatus = "warn"
	VerifyError VerifyStatus = "error"
)

// VerifyResult is one canton's probe record emitted by plate verify.
type VerifyResult struct {
	Canton     string       `json:"canton"`
	URL        string       `json:"url"`
	FinalURL   string       `json:"final_url,omitempty"`
	HTTPStatus int          `json:"http_status,omitempty"`
	Status     VerifyStatus `json:"status"`
	Reasons    []string     `json:"reasons,omitempty"`
	ElapsedMS  int64        `json:"elapsed_ms,omitempty"`
	CheckedAt  string       `json:"checked_at"`
}
