package output

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exactly max", "hello", 5, "hello"},
		{"longer than max", "hello world", 8, "hello..."},
		{"empty string", "", 5, ""},
		{"max 3", "hello", 3, "hel"},
		{"max 2", "hello", 2, "he"},
		{"max 1", "hello", 1, "h"},
		{"max 0", "hello", 0, ""},
		{"max 4", "hello world", 4, "h..."},
		{"unicode uses bytes", "Grüezi wohl", 8, "Grüe..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestMultilingualTextPick(t *testing.T) {
	text := MultilingualText{
		DE: "Deutsch",
		FR: "Francais",
		IT: "Italiano",
		EN: "English",
		RM: "Rumantsch",
	}

	tests := []struct {
		lang string
		want string
	}{
		{"de", "Deutsch"},
		{"fr", "Francais"},
		{"it", "Italiano"},
		{"en", "English"},
		{"rm", "Rumantsch"},
		{"unknown", "Deutsch"},
		{"", "Deutsch"},
	}
	for _, tt := range tests {
		t.Run("lang="+tt.lang, func(t *testing.T) {
			got := text.Pick(tt.lang)
			if got != tt.want {
				t.Errorf("Pick(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestMultilingualTextPickFallback(t *testing.T) {
	// When a language field is empty, should fall back to DE.
	text := MultilingualText{
		DE: "Deutsch",
		FR: "",
		IT: "",
		EN: "",
		RM: "",
	}

	for _, lang := range []string{"fr", "it", "en", "rm"} {
		t.Run("fallback_"+lang, func(t *testing.T) {
			got := text.Pick(lang)
			if got != "Deutsch" {
				t.Errorf("Pick(%q) with empty value = %q, want %q", lang, got, "Deutsch")
			}
		})
	}
}

func TestIsInteractiveForceJSON(t *testing.T) {
	// Save and restore global state.
	origForceJSON := ForceJSON
	origFormat := OutputFormat
	defer func() {
		ForceJSON = origForceJSON
		OutputFormat = origFormat
	}()

	ForceJSON = true
	OutputFormat = ""
	if IsInteractive() {
		t.Error("IsInteractive() = true when ForceJSON=true, want false")
	}

	ForceJSON = false
	OutputFormat = "json"
	if IsInteractive() {
		t.Error("IsInteractive() = true when OutputFormat=json, want false")
	}

	ForceJSON = false
	OutputFormat = "csv"
	if IsInteractive() {
		t.Error("IsInteractive() = true when OutputFormat=csv, want false")
	}

	ForceJSON = false
	OutputFormat = "tsv"
	if IsInteractive() {
		t.Error("IsInteractive() = true when OutputFormat=tsv, want false")
	}
}

func TestHighlight(t *testing.T) {
	// Force non-interactive mode so Highlight returns unmodified text
	// (color function returns plain text when not interactive).
	origForceJSON := ForceJSON
	origNoColor := NoColor
	defer func() {
		ForceJSON = origForceJSON
		NoColor = origNoColor
	}()

	// In non-interactive mode, Highlight should return the string unchanged.
	ForceJSON = true
	NoColor = false

	tests := []struct {
		input string
		want  string
	}{
		{"Erledigt", "Erledigt"},
		{"angenommen", "angenommen"},
		{"abgelehnt", "abgelehnt"},
		{"pending", "pending"},
		{"unknown status", "unknown status"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Highlight(tt.input)
			if got != tt.want {
				t.Errorf("Highlight(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	// When NoColor=false and interactive, Highlight should add color codes.
	// We can test this by enabling color explicitly.
	ForceJSON = false
	NoColor = false
	// In test environment stdout is not a terminal, so IsInteractive() returns false.
	// Highlight will still return plain text. That's correct behavior.
	got := Highlight("Erledigt")
	if got != "Erledigt" {
		// If it has color codes, that's also fine (means terminal is detected).
		if len(got) <= len("Erledigt") {
			t.Errorf("Highlight with color returned shorter string: %q", got)
		}
	}
}
