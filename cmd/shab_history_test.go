package cmd

import (
	"testing"

	"github.com/matthiasak/chli/api"
)

func TestChangeSummaryCreation(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Content: api.SHABXMLContent{
			Transaction: &api.SHABTransaction{Creation: &struct{}{}},
		},
	}
	if got := changeSummary(pub); got != "Neueintragung" {
		t.Errorf("got %q, want Neueintragung", got)
	}
}

func TestChangeSummaryDeletion(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Content: api.SHABXMLContent{
			Transaction: &api.SHABTransaction{Deletion: &struct{}{}},
		},
	}
	if got := changeSummary(pub); got != "Löschung" {
		t.Errorf("got %q, want Löschung", got)
	}
}

func TestChangeSummaryUpdate(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Content: api.SHABXMLContent{
			Transaction: &api.SHABTransaction{
				Update: &api.SHABTxUpdate{
					Changements: api.SHABChangements{SeatChanged: true, AddressChanged: true},
				},
			},
		},
	}
	if got := changeSummary(pub); got != "Mutation — seat, address" {
		t.Errorf("got %q", got)
	}
}

func TestChangeSummaryFallbackTitle(t *testing.T) {
	pub := &api.SHABPublicationXML{
		Meta: api.SHABXMLMeta{
			Title: &api.SHABXMLTitle{DE: "Some very long German title that exceeds forty characters easily"},
		},
	}
	got := changeSummary(pub)
	if len(got) > 40 {
		t.Errorf("expected truncation to 40 chars, got %q (%d)", got, len(got))
	}
}
