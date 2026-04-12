package api

import (
	"encoding/xml"
	"os"
	"testing"
)

func TestSHABParseHR02Fixture(t *testing.T) {
	data, err := os.ReadFile("testdata/shab_hr02.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var pub SHABPublicationXML
	if err := xml.Unmarshal(data, &pub); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if pub.Meta.PublicationNumber == "" {
		t.Errorf("missing publicationNumber")
	}
	if pub.Meta.Rubric != "HR" {
		t.Errorf("rubric = %q, want HR", pub.Meta.Rubric)
	}
	if pub.Meta.RegistrationOffice == nil || pub.Meta.RegistrationOffice.DisplayName == "" {
		t.Errorf("registrationOffice not parsed as struct: %+v", pub.Meta.RegistrationOffice)
	}
	if pub.Content.PublicationText.Body == "" {
		t.Errorf("publicationText body empty")
	}
	if pub.Content.CommonsNew == nil || pub.Content.CommonsNew.Company == nil {
		t.Fatalf("commonsNew.company missing")
	}
	if pub.Content.CommonsNew.Company.Name == "" {
		t.Errorf("company name empty")
	}
	if pub.Content.LastFosc == nil {
		t.Errorf("lastFosc missing")
	}
}
