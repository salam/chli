package output

import "testing"

func TestHyperlinkInteractive(t *testing.T) {
	prev := ForceJSON
	ForceJSON = false
	t.Cleanup(func() { ForceJSON = prev })

	got := hyperlinkFor(true, "https://example.com", "label")
	want := "\x1b]8;;https://example.com\x1b\\label\x1b]8;;\x1b\\"
	if got != want {
		t.Fatalf("interactive: got %q want %q", got, want)
	}
}

func TestHyperlinkNonInteractive(t *testing.T) {
	if got := hyperlinkFor(false, "https://example.com", "label"); got != "https://example.com" {
		t.Fatalf("non-interactive: got %q, want bare URL", got)
	}
}

func TestHyperlinkEmptyLabelUsesURL(t *testing.T) {
	got := hyperlinkFor(true, "https://example.com", "")
	if !containsSubstr(got, "https://example.com") {
		t.Fatalf("expected URL in body, got %q", got)
	}
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
