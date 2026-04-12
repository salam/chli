package cmd

import (
	"strings"
	"testing"

	"github.com/matthiasak/chli/api"
)

func TestDiffCommonsDetectsSeatChange(t *testing.T) {
	old := &api.SHABCommons{Company: &api.SHABCompany{Name: "Acme AG", Seat: "Zürich"}}
	new_ := &api.SHABCommons{Company: &api.SHABCompany{Name: "Acme AG", Seat: "Zug"}}
	lines := diffCommons(old, new_)
	if len(lines) != 1 {
		t.Fatalf("got %d diff lines, want 1: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0].Field, "Seat") ||
		lines[0].Old != "Zürich" || lines[0].New != "Zug" {
		t.Errorf("unexpected diff line: %+v", lines[0])
	}
}

func TestDiffCommonsIdenticalIsEmpty(t *testing.T) {
	c := &api.SHABCommons{Company: &api.SHABCompany{Name: "Acme", Seat: "Zug"}}
	if lines := diffCommons(c, c); len(lines) != 0 {
		t.Fatalf("expected no diffs, got %v", lines)
	}
}

func TestDiffCommonsNilSide(t *testing.T) {
	if lines := diffCommons(nil, nil); len(lines) != 0 {
		t.Fatalf("nil/nil should be empty, got %v", lines)
	}
}

func TestDiffCommonsAuditor(t *testing.T) {
	old := &api.SHABCommons{}
	new_ := &api.SHABCommons{
		Revision: &api.SHABRevision{
			RevisionCompany: &api.SHABRevisionCompany{Name: "Ernst & Young SA"},
		},
	}
	lines := diffCommons(old, new_)
	if len(lines) != 1 || lines[0].Field != "Auditor" ||
		lines[0].Old != "(none)" || lines[0].New != "Ernst & Young SA" {
		t.Errorf("unexpected: %+v", lines)
	}
}
