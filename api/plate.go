package api

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

// ValidCantons is the set of the 26 Swiss canton codes accepted as plate
// prefixes. Canton detection is a pure string operation; we never call an API
// for it.
var ValidCantons = map[string]struct{}{
	"AG": {}, "AI": {}, "AR": {}, "BE": {}, "BL": {}, "BS": {},
	"FR": {}, "GE": {}, "GL": {}, "GR": {}, "JU": {}, "LU": {},
	"NE": {}, "NW": {}, "OW": {}, "SG": {}, "SH": {}, "SO": {},
	"SZ": {}, "TG": {}, "TI": {}, "UR": {}, "VD": {}, "VS": {},
	"ZG": {}, "ZH": {},
}

// IsValidCanton reports whether code is one of the 26 Swiss canton codes.
// The comparison is case-insensitive.
func IsValidCanton(code string) bool {
	_, ok := ValidCantons[strings.ToUpper(strings.TrimSpace(code))]
	return ok
}

// SortedCantonCodes returns the 26 canton codes in deterministic alphabetical
// order. Used by verify and listings.
func SortedCantonCodes() []string {
	out := make([]string, 0, len(ValidCantons))
	for code := range ValidCantons {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// normalizePlate strips whitespace and hyphens and uppercases.
func normalizePlate(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case r == ' ' || r == '\t' || r == '-' || r == '_' || r == '\u00a0':
			continue
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParsePlate parses a user-supplied plate string. It accepts the
// canton-prefix form ("ZH123456", "ZH 123 456", "zh-123-456") and the
// body-only form (digits only, no canton). In the body-only case, Canton is
// empty and the caller must supply a canton list separately.
//
// It returns an error only when the input is empty, or when a 2-letter
// alphabetic prefix is present but is not one of the 26 valid codes.
// A non-digit body after a valid prefix yields a warning on the Plate, not
// an error — some special/diplomat plates contain letters.
func ParsePlate(raw string) (Plate, error) {
	p := Plate{Raw: raw}
	norm := normalizePlate(raw)
	if norm == "" {
		return p, fmt.Errorf("plate is empty")
	}
	p.Normalized = norm

	// Detect a 2-letter alphabetic prefix. Only treat it as a canton code
	// attempt if the first two runes are ASCII letters.
	if len(norm) >= 2 && isAlpha(norm[0]) && isAlpha(norm[1]) {
		code := norm[:2]
		if !IsValidCanton(code) {
			return p, fmt.Errorf("%q is not a valid Swiss canton code", code)
		}
		p.Canton = code
		body := norm[2:]
		p.Digits = body
		if body != "" && !isAllDigits(body) {
			p.Warnings = append(p.Warnings, fmt.Sprintf("plate body %q contains non-digit characters", body))
		}
	} else {
		// Body-only form: all characters must be plausible plate characters
		// (digits, or letters after the first position — we already know the
		// first is not alpha). We keep everything in Digits even if mixed;
		// ParsePlate is tolerant.
		p.Digits = norm
		if !isAllDigits(norm) {
			p.Warnings = append(p.Warnings, fmt.Sprintf("plate body %q contains non-digit characters", norm))
		}
	}
	return p, nil
}

func isAlpha(b byte) bool { return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') }
func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// prettyPlate formats a normalized plate as "XX 123 456" when the body is
// six digits, or as "XX 12345" / "XX123456" when shorter or non-numeric.
func prettyPlate(p Plate) string {
	if p.Canton == "" {
		return p.Normalized
	}
	body := p.Digits
	if body == "" {
		return p.Canton
	}
	if len(body) == 6 && isAllDigits(body) {
		return fmt.Sprintf("%s %s %s", p.Canton, body[:3], body[3:])
	}
	if len(body) == 5 && isAllDigits(body) {
		return fmt.Sprintf("%s %s %s", p.Canton, body[:2], body[2:])
	}
	if isAllDigits(body) {
		return fmt.Sprintf("%s %s", p.Canton, body)
	}
	return fmt.Sprintf("%s%s", p.Canton, body)
}

// DeeplinkFor renders the canton's deeplink template against the plate,
// substituting {{.PlateNormalized}}, {{.PlateRaw}}, {{.Canton}}. If the
// canton has no deeplink template, the plain URL is returned; if URL is
// also empty, an empty string is returned.
func DeeplinkFor(p Plate, c CantonEntry) (string, error) {
	tmplSrc := c.Halterauskunft.DeeplinkTemplate
	if tmplSrc == "" {
		return c.Halterauskunft.URL, nil
	}
	t, err := template.New("deeplink").Parse(tmplSrc)
	if err != nil {
		return "", fmt.Errorf("canton %s: parse deeplink template: %w", c.Code, err)
	}
	var buf bytes.Buffer
	data := struct {
		PlateNormalized string
		PlateRaw        string
		Canton          string
	}{
		PlateNormalized: p.Normalized,
		PlateRaw:        p.Raw,
		Canton:          c.Code,
	}
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("canton %s: execute deeplink template: %w", c.Code, err)
	}
	return buf.String(), nil
}

// RenderDispatch renders the human-readable dispatcher block for a single
// (plate, canton) combination. The shape follows the spec exactly.
func RenderDispatch(p Plate, c CantonEntry, lang string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Plate:         %s\n", prettyPlate(p))
	fmt.Fprintf(&b, "Canton:        %s (%s) — %s\n", c.Name(lang), c.Code, c.Authority.Name)

	h := c.Halterauskunft
	fmt.Fprintf(&b, "Service:       %s\n", describeService(h.Mode))

	// URL line: prefer a rendered deeplink, else landing URL, else form PDF.
	link, _ := DeeplinkFor(p, c)
	if link == "" {
		link = h.FormPDF
	}
	if link != "" {
		fmt.Fprintf(&b, "URL:           %s\n", link)
	}

	if h.Mode != ModeUnavailable {
		if h.CostCHF > 0 {
			fmt.Fprintf(&b, "Cost:          CHF %s per lookup\n", formatCHF(h.CostCHF))
		} else {
			fmt.Fprintf(&b, "Cost:          see authority for fee schedule\n")
		}
		if len(h.PaymentMethods) > 0 {
			fmt.Fprintf(&b, "Payment:       %s\n", strings.Join(formatPaymentMethods(h.PaymentMethods), ", "))
		}
		fmt.Fprintf(&b, "Auth:          %s\n", formatAuth(h.Auth))
		fmt.Fprintf(&b, "Processing:    %s\n", formatProcessing(h.Processing))
	}

	if h.LegalBasis != "" {
		fmt.Fprintf(&b, "Legal basis:   %s\n", h.LegalBasis)
	}

	if h.Queryable {
		fmt.Fprintf(&b, "Queryable:     yes — free self-serve, pass --query to open\n")
	}

	// Postal block for postal/mixed cantons.
	if (h.Mode == ModePostal || h.Mode == ModeMixed) && !c.Authority.Postal.IsEmpty() {
		fmt.Fprintf(&b, "Postal:        %s, %s %s\n",
			c.Authority.Postal.Street,
			c.Authority.Postal.Zip,
			c.Authority.Postal.City,
		)
	}

	if v := c.Verification; v.LastVerified != "" {
		src := ""
		if len(v.SourceURLs) > 0 {
			src = fmt.Sprintf(" (source: %s)", v.SourceURLs[0])
		}
		fmt.Fprintf(&b, "Data verified: %s%s\n", v.LastVerified, src)
	}

	if note := c.Note(lang); note != "" {
		fmt.Fprintf(&b, "Note:          %s\n", note)
	}

	return b.String()
}

// PrivacyNotice is the trailing reminder appended to TTY dispatcher output
// unless --no-privacy-notice is set.
const PrivacyNotice = "\nThis is a reference tool. Holder data is released only via the cantonal process above.\n"

func describeService(m HalterauskunftMode) string {
	switch m {
	case ModeOnline:
		return "Halterauskunft online"
	case ModePostal:
		return "Halterauskunft by post"
	case ModeMixed:
		return "Halterauskunft online or by post"
	case ModeUnavailable:
		return "Halterauskunft not published online"
	}
	return string(m)
}

func formatCHF(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%.2f", f)
}

func formatPaymentMethods(methods []string) []string {
	if len(methods) == 0 {
		return nil
	}
	out := make([]string, len(methods))
	for i, m := range methods {
		switch m {
		case "twint":
			out[i] = "Twint"
		case "mastercard":
			out[i] = "Mastercard"
		case "visa":
			out[i] = "Visa"
		case "amex":
			out[i] = "Amex"
		case "postfinance_card":
			out[i] = "PostFinance Card"
		case "postfinance":
			out[i] = "PostFinance"
		case "invoice":
			out[i] = "invoice"
		case "prepaid":
			out[i] = "prepaid"
		default:
			out[i] = m
		}
	}
	return out
}

func formatAuth(a AuthReqs) string {
	var parts []string
	switch a.Captcha {
	case CaptchaHCaptcha:
		parts = append(parts, "hCaptcha")
	case CaptchaReCaptcha:
		parts = append(parts, "reCAPTCHA")
	case CaptchaNone, "":
		// nothing
	}
	if a.SMS {
		parts = append(parts, "SMS confirmation")
	}
	if a.EmailConfirmation {
		parts = append(parts, "email confirmation")
	}
	if a.RequiresStatedReason {
		parts = append(parts, "stated reason (accident/damage/legal)")
	}
	if a.RequiresIdentification {
		parts = append(parts, "identity verification")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " + ")
}

func formatProcessing(p Processing) string {
	var typ string
	switch p.Typical {
	case ProcInstant:
		typ = "Instant"
	case ProcHours:
		typ = "Same day"
	case Proc1to3:
		typ = "1–3 business days"
	case Proc5to10:
		typ = "5–10 business days"
	default:
		if p.Typical == "" {
			typ = "unspecified"
		} else {
			typ = string(p.Typical)
		}
	}
	switch p.Delivery {
	case DeliveryPDFEmail:
		return typ + ", PDF emailed"
	case DeliveryPostal:
		return typ + ", by post"
	case DeliveryOnlinePortal:
		return typ + ", online portal"
	}
	return typ
}
