package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var (
	plateVerifyAll        bool
	plateVerifyCanton     string
	plateVerifyFailOnWarn bool
)

// plateVerifyConcurrency bounds parallel canton probes. 4 is polite enough for
// 26 government landing pages hit once a week.
const plateVerifyConcurrency = 4

var plateVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Probe cantonal Halterauskunft endpoints to detect data rot",
	Long: `Observationally probes each canton's Halterauskunft landing page, records
status + body heuristics, and emits a JSON report. Never submits the form,
never solves captcha. Used by CI on release and weekly.`,
	RunE: runPlateVerify,
}

func runPlateVerify(cmd *cobra.Command, args []string) error {
	cantons, err := api.LoadCantons()
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}

	// Resolve the set to probe.
	var codes []string
	switch {
	case strings.TrimSpace(plateVerifyCanton) != "":
		for _, r := range strings.Split(plateVerifyCanton, ",") {
			code := strings.ToUpper(strings.TrimSpace(r))
			if code == "" {
				continue
			}
			if !api.IsValidCanton(code) {
				output.Error(fmt.Sprintf("%q is not a valid Swiss canton code", code))
				os.Exit(1)
			}
			codes = append(codes, code)
		}
	case plateVerifyAll:
		codes = api.SortedCantonCodes()
	default:
		// Default to --all when neither flag is set.
		codes = api.SortedCantonCodes()
	}
	sort.Strings(codes)

	// Build worker pool.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	jobs := make(chan api.CantonEntry, len(codes))
	type indexed struct {
		idx int
		res api.VerifyResult
	}
	results := make(chan indexed, len(codes))

	var wg sync.WaitGroup
	for w := 0; w < plateVerifyConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				res := api.VerifyCanton(ctx, entry)
				results <- indexed{idx: indexOfCode(codes, entry.Code), res: res}
			}
		}()
	}

	for _, code := range codes {
		entry, ok := cantons[code]
		if !ok {
			output.Error(fmt.Sprintf("canton %s: not loaded", code))
			os.Exit(1)
		}
		jobs <- entry
	}
	close(jobs)
	wg.Wait()
	close(results)

	ordered := make([]api.VerifyResult, len(codes))
	for r := range results {
		ordered[r.idx] = r.res
	}

	// Emit output.
	interactive := output.IsInteractive() && !output.ForceJSON
	if interactive {
		headers := []string{"Canton", "Status", "HTTP", "URL", "Reasons"}
		rows := make([][]string, 0, len(ordered))
		for _, r := range ordered {
			reasons := ""
			if len(r.Reasons) > 0 {
				reasons = strings.Join(r.Reasons, "; ")
			}
			http := ""
			if r.HTTPStatus > 0 {
				http = fmt.Sprintf("%d", r.HTTPStatus)
			}
			rows = append(rows, []string{r.Canton, string(r.Status), http, r.URL, output.Truncate(reasons, 80)})
		}
		output.Table(headers, rows)
	} else {
		switch output.OutputFormat {
		case "yaml", "yml":
			output.YAML(ordered)
		case "md", "markdown":
			headers := []string{"Canton", "Status", "HTTP", "URL", "Reasons"}
			rows := make([][]string, 0, len(ordered))
			for _, r := range ordered {
				reasons := strings.Join(r.Reasons, "; ")
				http := ""
				if r.HTTPStatus > 0 {
					http = fmt.Sprintf("%d", r.HTTPStatus)
				}
				rows = append(rows, []string{r.Canton, string(r.Status), http, r.URL, reasons})
			}
			output.Markdown(headers, rows)
		case "csv":
			writeVerifyCSV(ordered, false)
		case "tsv":
			writeVerifyCSV(ordered, true)
		default:
			output.JSON(ordered)
		}
	}

	// Exit-code policy.
	anyError, anyWarn := false, false
	for _, r := range ordered {
		switch r.Status {
		case api.VerifyError:
			anyError = true
		case api.VerifyWarn:
			anyWarn = true
		}
	}
	if anyError {
		os.Exit(1)
	}
	if plateVerifyFailOnWarn && anyWarn {
		os.Exit(2)
	}
	return nil
}

func writeVerifyCSV(rs []api.VerifyResult, tsv bool) {
	headers := []string{"canton", "status", "http", "url", "final_url", "elapsed_ms", "reasons"}
	rows := make([][]string, 0, len(rs))
	for _, r := range rs {
		http := ""
		if r.HTTPStatus > 0 {
			http = fmt.Sprintf("%d", r.HTTPStatus)
		}
		rows = append(rows, []string{
			r.Canton, string(r.Status), http,
			r.URL, r.FinalURL,
			fmt.Sprintf("%d", r.ElapsedMS),
			strings.Join(r.Reasons, "; "),
		})
	}
	if tsv {
		output.TSV(headers, rows)
	} else {
		output.CSV(headers, rows)
	}
}

func indexOfCode(codes []string, code string) int {
	for i, c := range codes {
		if c == code {
			return i
		}
	}
	return 0
}

func init() {
	plateVerifyCmd.Flags().BoolVar(&plateVerifyAll, "all", false, "Probe all 26 cantons (default)")
	plateVerifyCmd.Flags().StringVar(&plateVerifyCanton, "canton", "", "Comma-separated canton codes to probe")
	plateVerifyCmd.Flags().BoolVar(&plateVerifyFailOnWarn, "fail-on-warn", false, "Exit 2 when any canton reports a warning")
}
