package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

var ForceJSON bool
var NoColor bool
var Lang string = "de"

func IsInteractive() bool {
	if ForceJSON {
		return false
	}
	fi, _ := os.Stdout.Stat()
	return fi.Mode()&os.ModeCharDevice != 0
}

func Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

func JSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func Section(title string) {
	fmt.Printf("\n--- %s ---\n", title)
}

func Error(msg string) {
	if IsInteractive() {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	} else {
		JSON(map[string]string{"error": msg})
	}
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

type MultilingualText struct {
	DE string `json:"de"`
	FR string `json:"fr"`
	IT string `json:"it"`
	EN string `json:"en"`
	RM string `json:"rm,omitempty"`
}

func (t MultilingualText) Pick(lang string) string {
	var val string
	switch lang {
	case "fr":
		val = t.FR
	case "it":
		val = t.IT
	case "en":
		val = t.EN
	case "rm":
		val = t.RM
	default:
		val = t.DE
	}
	if val == "" {
		return t.DE
	}
	return val
}
