package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

var ForceJSON bool
var NoColor bool
var Quiet bool
var Lang string = "de"
var OutputFormat string // "json", "csv", "tsv", or "" (default table)
var Verbose bool
var Debug bool
var Columns string // Comma-separated list of columns to display

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

func color(c, s string) string {
	if NoColor || !IsInteractive() {
		return s
	}
	return c + s + colorReset
}

func IsInteractive() bool {
	switch OutputFormat {
	case "json", "csv", "tsv", "yaml", "yml", "md", "markdown":
		return false
	}
	if ForceJSON {
		return false
	}
	fi, _ := os.Stdout.Stat()
	return fi.Mode()&os.ModeCharDevice != 0
}

// usePager returns true if we should pipe output through a pager.
func usePager() bool {
	if Quiet || !IsInteractive() {
		return false
	}
	// Only use pager if PAGER env is set or less/more is available
	if p := os.Getenv("PAGER"); p != "" {
		return true
	}
	_, err := exec.LookPath("less")
	return err == nil
}

// pagerWriter returns a writer that pipes to a pager if appropriate.
// The caller must call the returned cleanup function when done.
func pagerWriter(minLines int, rows int) (io.Writer, func()) {
	if rows < minLines || !usePager() {
		return os.Stdout, func() {}
	}

	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	args := []string{}
	if pager == "less" {
		args = []string{"-FRSX"}
	}

	cmd := exec.Command(pager, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	w, err := cmd.StdinPipe()
	if err != nil {
		return os.Stdout, func() {}
	}

	if err := cmd.Start(); err != nil {
		return os.Stdout, func() {}
	}

	return w, func() {
		w.Close()
		cmd.Wait()
	}
}

// FilterColumns filters headers and rows to only include the columns specified
// in the Columns variable (comma-separated, case-insensitive). If Columns is
// empty, all columns are returned unchanged.
func FilterColumns(headers []string, rows [][]string) ([]string, [][]string) {
	if Columns == "" {
		return headers, rows
	}

	wanted := strings.Split(Columns, ",")
	var indices []int
	for _, w := range wanted {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		for i, h := range headers {
			if strings.EqualFold(h, w) {
				indices = append(indices, i)
				break
			}
		}
	}

	if len(indices) == 0 {
		return headers, rows
	}

	newHeaders := make([]string, len(indices))
	for i, idx := range indices {
		newHeaders[i] = headers[idx]
	}

	newRows := make([][]string, len(rows))
	for r, row := range rows {
		newRow := make([]string, len(indices))
		for i, idx := range indices {
			if idx < len(row) {
				newRow[i] = row[idx]
			}
		}
		newRows[r] = newRow
	}

	return newHeaders, newRows
}

func Table(headers []string, rows [][]string) {
	headers, rows = FilterColumns(headers, rows)
	if Quiet {
		// Data rows only, no headers
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		for _, row := range rows {
			fmt.Fprintln(w, strings.Join(row, "\t"))
		}
		w.Flush()
		return
	}

	out, cleanup := pagerWriter(50, len(rows))
	defer cleanup()

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	// Colorize header
	coloredHeaders := make([]string, len(headers))
	for i, h := range headers {
		coloredHeaders[i] = color(colorBold+colorCyan, h)
	}
	fmt.Fprintln(w, strings.Join(coloredHeaders, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

func CSV(headers []string, rows [][]string) {
	w := csv.NewWriter(os.Stdout)
	if !Quiet {
		w.Write(headers)
	}
	for _, row := range rows {
		w.Write(row)
	}
	w.Flush()
}

func TSV(headers []string, rows [][]string) {
	w := csv.NewWriter(os.Stdout)
	w.Comma = '\t'
	if !Quiet {
		w.Write(headers)
	}
	for _, row := range rows {
		w.Write(row)
	}
	w.Flush()
}

func JSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// YAML emits any value as YAML by round-tripping through JSON (to honour
// json tags) and then recursively rendering the generic tree. No external
// dependency — sufficient for the flat/nested primitive-heavy shapes the
// CLI produces.
func YAML(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintln(os.Stderr, "yaml: marshal:", err)
		return
	}
	var tree any
	if err := json.Unmarshal(b, &tree); err != nil {
		fmt.Fprintln(os.Stderr, "yaml: unmarshal:", err)
		return
	}
	writeYAML(os.Stdout, tree, 0, false)
}

func writeYAML(w io.Writer, v any, indent int, inList bool) {
	pad := strings.Repeat("  ", indent)
	switch x := v.(type) {
	case nil:
		fmt.Fprintln(w, "null")
	case map[string]any:
		if len(x) == 0 {
			fmt.Fprintln(w, "{}")
			return
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			prefix := pad
			if i == 0 && inList {
				prefix = ""
			}
			val := x[k]
			if isScalar(val) {
				fmt.Fprintf(w, "%s%s: %s\n", prefix, yamlKey(k), yamlScalar(val))
			} else if isEmpty(val) {
				fmt.Fprintf(w, "%s%s: %s\n", prefix, yamlKey(k), emptyLiteral(val))
			} else {
				fmt.Fprintf(w, "%s%s:\n", prefix, yamlKey(k))
				writeYAML(w, val, indent+1, false)
			}
		}
	case []any:
		if len(x) == 0 {
			fmt.Fprintln(w, "[]")
			return
		}
		for _, item := range x {
			if isScalar(item) {
				fmt.Fprintf(w, "%s- %s\n", pad, yamlScalar(item))
			} else {
				fmt.Fprintf(w, "%s- ", pad)
				writeYAML(w, item, indent+1, true)
			}
		}
	default:
		if reflect.ValueOf(v).Kind() == reflect.Invalid {
			fmt.Fprintln(w, "null")
			return
		}
		fmt.Fprintln(w, yamlScalar(v))
	}
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, bool, string, float64, float32, int, int64, int32:
		return true
	}
	return false
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		return len(x) == 0
	case []any:
		return len(x) == 0
	}
	return false
}

func emptyLiteral(v any) string {
	if _, ok := v.([]any); ok {
		return "[]"
	}
	return "{}"
}

func yamlScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return yamlString(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func yamlKey(k string) string {
	if k == "" || strings.ContainsAny(k, ": #\n\"'{}[],&*!|>%@`") {
		return strconv.Quote(k)
	}
	return k
}

func yamlString(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, "\n\"\\") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return strconv.Quote(s)
	}
	// Reserved words / ambiguous scalars need quoting
	switch strings.ToLower(s) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return strconv.Quote(s)
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return strconv.Quote(s)
	}
	if strings.ContainsAny(s, ":#") {
		return strconv.Quote(s)
	}
	return s
}

// Markdown emits a GitHub-flavoured markdown table.
func Markdown(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	escape := func(s string) string {
		s = strings.ReplaceAll(s, "|", `\|`)
		s = strings.ReplaceAll(s, "\n", " ")
		return s
	}
	esc := make([]string, len(headers))
	for i, h := range headers {
		esc[i] = escape(h)
	}
	fmt.Fprintf(os.Stdout, "| %s |\n", strings.Join(esc, " | "))
	sep := make([]string, len(headers))
	for i := range sep {
		sep[i] = "---"
	}
	fmt.Fprintf(os.Stdout, "| %s |\n", strings.Join(sep, " | "))
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range cells {
			if i < len(row) {
				cells[i] = escape(row[i])
			}
		}
		fmt.Fprintf(os.Stdout, "| %s |\n", strings.Join(cells, " | "))
	}
}

// Render outputs data using the appropriate format based on flags and TTY detection.
// This is the primary output function commands should use.
func Render(headers []string, rows [][]string, jsonData any) {
	headers, rows = FilterColumns(headers, rows)
	switch OutputFormat {
	case "csv":
		CSV(headers, rows)
	case "tsv":
		TSV(headers, rows)
	case "json":
		JSON(jsonData)
	case "yaml", "yml":
		YAML(jsonData)
	case "md", "markdown":
		Markdown(headers, rows)
	default:
		if ForceJSON || !isTerminal() {
			JSON(jsonData)
		} else {
			Table(headers, rows)
		}
	}
}

func isTerminal() bool {
	fi, _ := os.Stdout.Stat()
	return fi.Mode()&os.ModeCharDevice != 0
}

func Section(title string) {
	if Quiet {
		return
	}
	fmt.Printf("\n%s\n", color(colorBold+colorYellow, "--- "+title+" ---"))
}

func Error(msg string) {
	if IsInteractive() {
		fmt.Fprintf(os.Stderr, "%s %s\n", color(colorRed, "Error:"), msg)
	} else {
		JSON(map[string]string{"error": msg})
	}
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// Highlight applies semantic coloring for common Swiss government data patterns.
func Highlight(s string) string {
	if NoColor || !IsInteractive() {
		return s
	}
	lower := strings.ToLower(s)
	// Status highlighting
	switch {
	case lower == "erledigt" || lower == "angenommen" || lower == "adopted" || lower == "yes" || lower == "active":
		return color(colorGreen, s)
	case lower == "abgelehnt" || lower == "rejected" || lower == "no" || lower == "inactive":
		return color(colorRed, s)
	case lower == "im rat noch nicht behandelt" || lower == "pending" || lower == "hängig":
		return color(colorYellow, s)
	}
	return s
}

// DateColor returns a date string with subtle coloring.
func DateColor(s string) string {
	return color(colorGray, s)
}

// Debugf prints debug output if --debug is enabled.
func Debugf(format string, args ...any) {
	if Debug {
		fmt.Fprintf(os.Stderr, color(colorGray, "[DEBUG] ")+format+"\n", args...)
	}
}

// Verbosef prints verbose output if --verbose or --debug is enabled.
func Verbosef(format string, args ...any) {
	if Verbose || Debug {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
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
