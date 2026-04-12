package api

import "testing"

func TestSHABPublicationURL(t *testing.T) {
	got := SHABPublicationURL("c27f4008-2dbc-4da4-af9e-9be31ea16ec2")
	want := "https://shab.ch/#!/search/publications/detail/c27f4008-2dbc-4da4-af9e-9be31ea16ec2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if SHABPublicationURL("") != "" {
		t.Fatalf("empty id should yield empty url")
	}
}
