package api

import "testing"

func TestLangURI(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"de", "DEU"},
		{"fr", "FRA"},
		{"it", "ITA"},
		{"en", "ENG"},
		{"rm", "ROH"},
		{"", "DEU"},
		{"unknown", "DEU"},
	}
	for _, tt := range tests {
		t.Run("lang="+tt.lang, func(t *testing.T) {
			got := LangURI(tt.lang)
			if got != tt.want {
				t.Errorf("LangURI(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestShortenURI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"with slash", "http://example.com/path/segment", "segment"},
		{"with hash", "http://example.com/path#fragment", "path#fragment"},
		{"empty", "", ""},
		{"no separator", "justAString", "justAString"},
		{"trailing slash", "http://example.com/", "http://example.com/"},
		{"slash and hash", "http://example.com/path/to#thing", "to#thing"},
		{"only hash", "foo#bar", "bar"},
		{"deep path", "https://fedlex.data.admin.ch/vocabulary/legal-institution/2", "2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenURI(tt.input)
			if got != tt.want {
				t.Errorf("shortenURI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
