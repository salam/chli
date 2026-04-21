package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var (
	plateCantonFlag       string
	plateOpenFlag         bool
	plateLangFlag         string
	plateNoPrivacyNotice  bool
)

var plateCmd = &cobra.Command{
	Use:   "plate <plate>",
	Short: "Dispatch a Swiss number plate to the correct cantonal Halterauskunft process",
	Long: `Given a Swiss number plate, print where and how to request holder information
(Halterauskunft) for the correct cantonal authority. chli never submits the
form, never solves a captcha, and never returns personal data.

Accepted plate forms:
  chli plate ZH123456
  chli plate "ZH 123 456"
  chli plate zh-123-456
  chli plate 120120 --canton AG,AI,FR
`,
	Args: cobra.ExactArgs(1),
	RunE: runPlate,
}

func runPlate(cmd *cobra.Command, args []string) error {
	lang := plateLangFlag
	if lang == "" {
		lang = output.Lang
	}

	p, err := api.ParsePlate(args[0])
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}

	cantons, err := api.LoadCantons()
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}

	// Determine the set of cantons to dispatch to.
	codes, err := resolvePlateCantons(p, plateCantonFlag)
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}

	entries := make([]api.CantonEntry, 0, len(codes))
	for _, code := range codes {
		entry, ok := cantons[code]
		if !ok {
			output.Error(fmt.Sprintf("canton %s: no data loaded (this is a bug — report it)", code))
			os.Exit(1)
		}
		entries = append(entries, entry)
	}

	// Warnings first (TTY only).
	interactive := output.IsInteractive() && !output.ForceJSON
	if interactive {
		for _, w := range p.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
	}

	// JSON / other machine-readable formats: emit the structured record(s).
	// The spec is silent on single-vs-list; single canton -> object, multi -> array.
	if !interactive {
		payload := buildPlateJSONPayload(p, entries)
		switch output.OutputFormat {
		case "yaml", "yml":
			output.YAML(payload)
		case "md", "markdown":
			// For md we render the same dispatcher text — Markdown-safe
			// since it's just prose.
			for i, e := range entries {
				if i > 0 {
					fmt.Println()
					fmt.Println("---")
					fmt.Println()
				}
				fmt.Print(api.RenderDispatch(p, e, lang))
			}
		case "csv":
			// CSV is awkward for the dispatcher shape — emit a minimal summary.
			writePlateCSV(p, entries, lang, false)
		case "tsv":
			writePlateCSV(p, entries, lang, true)
		default:
			output.JSON(payload)
		}
		// --open still honoured in non-TTY mode for scripts that want it.
		maybeOpenPlateLinks(p, entries)
		return nil
	}

	// Interactive output.
	for i, e := range entries {
		if i > 0 {
			fmt.Println()
		}
		fmt.Print(api.RenderDispatch(p, e, lang))
	}
	if !plateNoPrivacyNotice {
		fmt.Print(api.PrivacyNotice)
	}

	maybeOpenPlateLinks(p, entries)
	return nil
}

// buildPlateJSONPayload constructs the machine-readable payload. Single canton
// returns the entry directly; multiple returns a wrapper including the plate.
func buildPlateJSONPayload(p api.Plate, entries []api.CantonEntry) any {
	type link struct {
		Canton   string           `json:"canton"`
		URL      string           `json:"url,omitempty"`
		Entry    api.CantonEntry  `json:"entry"`
	}
	links := make([]link, 0, len(entries))
	for _, e := range entries {
		url, _ := api.DeeplinkFor(p, e)
		links = append(links, link{Canton: e.Code, URL: url, Entry: e})
	}
	return map[string]any{
		"plate":   p,
		"matches": links,
	}
}

func writePlateCSV(p api.Plate, entries []api.CantonEntry, lang string, tsv bool) {
	headers := []string{"canton", "name", "authority", "mode", "url", "cost_chf"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		url, _ := api.DeeplinkFor(p, e)
		rows = append(rows, []string{
			e.Code,
			e.Name(lang),
			e.Authority.Name,
			string(e.Halterauskunft.Mode),
			url,
			fmt.Sprintf("%g", e.Halterauskunft.CostCHF),
		})
	}
	if tsv {
		output.TSV(headers, rows)
	} else {
		output.CSV(headers, rows)
	}
}

func resolvePlateCantons(p api.Plate, flag string) ([]string, error) {
	// Explicit --canton flag wins, regardless of prefix.
	if strings.TrimSpace(flag) != "" {
		raw := strings.Split(flag, ",")
		var out []string
		seen := map[string]bool{}
		for _, r := range raw {
			code := strings.ToUpper(strings.TrimSpace(r))
			if code == "" {
				continue
			}
			if !api.IsValidCanton(code) {
				return nil, fmt.Errorf("%q is not a valid Swiss canton code", code)
			}
			if seen[code] {
				continue
			}
			seen[code] = true
			out = append(out, code)
		}
		sort.Strings(out)
		if len(out) == 0 {
			return nil, fmt.Errorf("--canton flag resolved to empty list")
		}
		return out, nil
	}
	if p.Canton != "" {
		return []string{p.Canton}, nil
	}
	return nil, fmt.Errorf("no canton prefix in plate %q and no --canton flag; pass --canton XX[,YY,...] to dispatch", p.Raw)
}

// maybeOpenPlateLinks launches the OS default browser for each resolved URL
// when --open is passed.
func maybeOpenPlateLinks(p api.Plate, entries []api.CantonEntry) {
	if !plateOpenFlag {
		return
	}
	for _, e := range entries {
		url, _ := api.DeeplinkFor(p, e)
		if url == "" {
			continue
		}
		if err := openURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "open: %s: %v\n", url, err)
		}
	}
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func init() {
	plateCmd.Flags().StringVar(&plateCantonFlag, "canton", "", "Comma-separated canton codes to dispatch to (required for fulltext form)")
	plateCmd.Flags().BoolVar(&plateOpenFlag, "open", false, "Open the resolved URL in the default browser")
	plateCmd.Flags().StringVar(&plateLangFlag, "lang", "", "Override the global --lang for this command (de|fr|it|en|rm)")
	plateCmd.Flags().BoolVar(&plateNoPrivacyNotice, "no-privacy-notice", false, "Suppress the trailing privacy reminder")

	plateCmd.AddCommand(plateVerifyCmd)
	rootCmd.AddCommand(plateCmd)
}
