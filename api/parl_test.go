package api

import (
	"strings"
	"testing"
	"time"

	"github.com/matthiasak/chli/output"
)

func TestODataQueryBuild(t *testing.T) {
	// Save and restore global state.
	origLang := output.Lang
	defer func() { output.Lang = origLang }()
	output.Lang = "de"

	tests := []struct {
		name     string
		build    func() string
		contains []string
	}{
		{
			name: "basic query",
			build: func() string {
				return NewODataQuery("Business").Build()
			},
			contains: []string{
				"Business?",
				"%24format=json",
				"Language+eq+%27DE%27",
			},
		},
		{
			name: "with filter",
			build: func() string {
				return NewODataQuery("Business").Filter("ID eq 1234").Build()
			},
			contains: []string{
				"ID+eq+1234",
				"Language+eq+%27DE%27",
			},
		},
		{
			name: "with select",
			build: func() string {
				return NewODataQuery("Business").Select("ID", "Title").Build()
			},
			contains: []string{
				"%24select=ID%2CTitle",
			},
		},
		{
			name: "with top and skip",
			build: func() string {
				return NewODataQuery("Business").Top(10).Skip(20).Build()
			},
			contains: []string{
				"%24top=10",
				"%24skip=20",
			},
		},
		{
			name: "with orderby",
			build: func() string {
				return NewODataQuery("Business").OrderBy("SubmissionDate desc").Build()
			},
			contains: []string{
				"%24orderby=SubmissionDate+desc",
			},
		},
		{
			name: "with expand",
			build: func() string {
				return NewODataQuery("Business").Expand("Votes").Build()
			},
			contains: []string{
				"%24expand=Votes",
			},
		},
		{
			name: "combined",
			build: func() string {
				return NewODataQuery("Vote").
					Filter("IdVote eq 42").
					Select("ID", "Subject").
					Top(5).
					OrderBy("Date desc").
					Build()
			},
			contains: []string{
				"Vote?",
				"IdVote+eq+42",
				"%24select=ID%2CSubject",
				"%24top=5",
				"%24orderby=Date+desc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.build()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("Build() = %q, missing %q", got, want)
				}
			}
		})
	}
}

func TestODataQueryBuildLanguage(t *testing.T) {
	origLang := output.Lang
	defer func() { output.Lang = origLang }()

	tests := []struct {
		lang     string
		contains string
	}{
		{"de", "Language+eq+%27DE%27"},
		{"fr", "Language+eq+%27FR%27"},
		{"it", "Language+eq+%27IT%27"},
		{"en", "Language+eq+%27EN%27"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			output.Lang = tt.lang
			got := NewODataQuery("Business").Build()
			if !strings.Contains(got, tt.contains) {
				t.Errorf("Build() with lang=%q = %q, missing %q", tt.lang, got, tt.contains)
			}
		})
	}
}

func TestParseODataDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid date", "/Date(1609459200000)/", "2021-01-01"},
		{"empty", "", ""},
		{"invalid format", "not-a-date", "not-a-date"},
		{"plain date string", "2021-01-01", "2021-01-01"},
		{"zero epoch", "/Date(0)/", "1970-01-01"},
		{"negative date", "/Date(-86400000)/", "1969-12-31"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseODataDate(tt.input)
			if got != tt.want {
				t.Errorf("ParseODataDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePeriod(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, got time.Time)
	}{
		{
			name:  "today",
			input: "today",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
				if !got.Equal(want) {
					t.Errorf("got %v, want %v", got, want)
				}
			},
		},
		{
			name:  "month",
			input: "month",
			check: func(t *testing.T, got time.Time) {
				want := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
				if !got.Equal(want) {
					t.Errorf("got %v, want %v", got, want)
				}
			},
		},
		{
			name:  "week",
			input: "week",
			check: func(t *testing.T, got time.Time) {
				// Should be a Monday at midnight.
				if got.Weekday() != time.Monday {
					t.Errorf("got weekday %v, want Monday", got.Weekday())
				}
				if got.Hour() != 0 || got.Minute() != 0 {
					t.Errorf("got %v, want midnight", got)
				}
			},
		},
		{
			name:  "last 30 days",
			input: "last 30 days",
			check: func(t *testing.T, got time.Time) {
				diff := time.Since(got)
				// Should be approximately 30 days ago.
				if diff < 29*24*time.Hour || diff > 31*24*time.Hour {
					t.Errorf("got %v, expected ~30 days ago", got)
				}
			},
		},
		{
			name:  "specific date",
			input: "2024-01-01",
			check: func(t *testing.T, got time.Time) {
				want, _ := time.Parse("2006-01-02", "2024-01-01")
				if !got.Equal(want) {
					t.Errorf("got %v, want %v", got, want)
				}
			},
		},
		{
			name:    "invalid",
			input:   "garbage",
			wantErr: true,
		},
		{
			name:  "this week",
			input: "this week",
			check: func(t *testing.T, got time.Time) {
				if got.Weekday() != time.Monday {
					t.Errorf("got weekday %v, want Monday", got.Weekday())
				}
			},
		},
		{
			name:  "this month",
			input: "this month",
			check: func(t *testing.T, got time.Time) {
				if got.Day() != 1 {
					t.Errorf("got day %d, want 1", got.Day())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePeriod(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("ParsePeriod() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePeriod() unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestTranslateVoteText(t *testing.T) {
	origLang := output.Lang
	defer func() { output.Lang = origLang }()

	tests := []struct {
		name   string
		lang   string
		french string
		want   string
	}{
		{"de translation", "de", "Vote final", "Schlussabstimmung"},
		{"it translation", "it", "Vote final", "Votazione finale"},
		{"en translation", "en", "Vote final", "Final vote"},
		{"fr passthrough", "fr", "Vote final", "Vote final"},
		{"unknown text", "de", "Something unknown", "Something unknown"},
		{"empty text", "de", "", ""},
		{"fr empty", "fr", "", ""},
		{"adopt motion de", "de", "Adopter la motion", "Annahme der Motion"},
		{"reject draft en", "en", "Rejeter le projet", "Reject the draft"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output.Lang = tt.lang
			got := TranslateVoteText(tt.french)
			if got != tt.want {
				t.Errorf("TranslateVoteText(%q) with lang=%q = %q, want %q",
					tt.french, tt.lang, got, tt.want)
			}
		})
	}
}
