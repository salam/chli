package cmd

import (
	"fmt"
	"strings"

	"github.com/matthiasak/chli/api"
)

// diffLine is a single field-level difference between two SHABCommons.
type diffLine struct {
	Field string
	Old   string
	New   string
}

// diffCommons returns a list of human-readable diff lines between two commons
// blocks. Fields absent on one side are rendered as "(none)".
func diffCommons(a, b *api.SHABCommons) []diffLine {
	var out []diffLine
	aCo, bCo := companyOf(a), companyOf(b)
	push := func(field, old, new string) {
		if old == new {
			return
		}
		if old == "" {
			old = "(none)"
		}
		if new == "" {
			new = "(none)"
		}
		out = append(out, diffLine{Field: field, Old: old, New: new})
	}

	push("Name", aCo.Name, bCo.Name)
	push("UID", aCo.UID, bCo.UID)
	push("Seat", aCo.Seat, bCo.Seat)
	push("Legal form", aCo.LegalForm, bCo.LegalForm)
	push("Address", addressOneLine(aCo.Address), addressOneLine(bCo.Address))
	push("Purpose", purposeOf(a), purposeOf(b))
	push("Auditor", auditorOf(a), auditorOf(b))
	return out
}

func companyOf(c *api.SHABCommons) api.SHABCompany {
	if c == nil || c.Company == nil {
		return api.SHABCompany{}
	}
	return *c.Company
}

func purposeOf(c *api.SHABCommons) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Purpose)
}

func auditorOf(c *api.SHABCommons) string {
	if c == nil || c.Revision == nil || c.Revision.RevisionCompany == nil {
		return ""
	}
	return c.Revision.RevisionCompany.Name
}

func addressOneLine(a *api.SHABAddress) string {
	if a == nil {
		return ""
	}
	parts := []string{}
	if a.Street != "" {
		s := a.Street
		if a.HouseNumber != "" {
			s += " " + a.HouseNumber
		}
		parts = append(parts, s)
	}
	if a.SwissZipCode != "" || a.Town != "" {
		parts = append(parts, strings.TrimSpace(a.SwissZipCode+" "+a.Town))
	}
	return strings.Join(parts, ", ")
}

// printDiff writes the diff lines to stdout in before/after format.
func printDiff(lines []diffLine) {
	if len(lines) == 0 {
		fmt.Println("No field-level changes (see Changes line).")
		return
	}
	for _, d := range lines {
		fmt.Printf("%-10s %s\n", d.Field+":", d.Old)
		fmt.Printf("%-10s → %s\n", "", d.New)
	}
}
