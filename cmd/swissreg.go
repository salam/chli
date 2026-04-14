package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var swissregCmd = &cobra.Command{
	Use:   "swissreg",
	Short: "Swiss IP register: trademarks, patents, designs",
	Long: `Search the Swiss Federal Institute of Intellectual Property (IPI)
public register via the swissreg.ch unauthenticated search backend.

Examples:

  chli swissreg trademark '"Ovomaltine"'             # exact phrase
  chli swissreg trademark Nestle --status aktiv --class 30
  chli swissreg trademark Nestle --filed-after 2020-01-01 --max 50
  chli swissreg patent Widerstand --office CH
  chli swissreg detail chmarke 1206422825
  chli swissreg image 570105 --out logo.png

Pagination beyond ` + "`--max 64`" + ` is not supported: the swissreg backend
uses an opaque Transit+JSON cursor that we don't decode. Refine the query
instead.`,
}

// swissregSearchFlags are shared by all the shortcut search subcommands.
type swissregSearchFlags struct {
	Max         int
	Status      string
	Class       string
	FiledAfter  string
	FiledBefore string
	Office      string
}

func addSearchFlags(c *cobra.Command, f *swissregSearchFlags) {
	c.Flags().IntVar(&f.Max, "max", 16, "Max results (1-64)")
	c.Flags().StringVar(&f.Status, "status", "", "Filter by status: aktiv, geloescht")
	c.Flags().StringVar(&f.Class, "class", "", "Filter by Nice/Locarno/IPC class (e.g. 30, or 5,30,32)")
	c.Flags().StringVar(&f.FiledAfter, "filed-after", "", "Filing date lower bound (YYYY-MM-DD)")
	c.Flags().StringVar(&f.FiledBefore, "filed-before", "", "Filing date upper bound (YYYY-MM-DD)")
	c.Flags().StringVar(&f.Office, "office", "", "Office-of-origin country code (e.g. CH, DE, AT)")
}

func buildFilters(target string, f *swissregSearchFlags) (map[string][]string, error) {
	filters := map[string][]string{}

	if f.Status != "" {
		key := "schutztitelstatus__type_i18n"
		val := "schutztitel.enum.schutztitelstatus." + strings.ToLower(f.Status)
		filters[key] = []string{val}
	}

	if f.Class != "" {
		// Class filter key differs per target (but the indexed text field is
		// consistent for chmarke). We only expose it for trademarks/designs.
		key := "wdlklassennummer__type_text_mv"
		var vals []string
		for _, c := range strings.Split(f.Class, ",") {
			if c = strings.TrimSpace(c); c != "" {
				vals = append(vals, c)
			}
		}
		filters[key] = vals
	}

	if f.FiledAfter != "" || f.FiledBefore != "" {
		from := dateOrStar(f.FiledAfter, true)
		to := dateOrStar(f.FiledBefore, false)
		filters["hinterlegungsdatum__type_date"] = []string{"[" + from + " TO " + to + "]"}
	}

	if f.Office != "" {
		filters["officeorigincode__type_string"] = []string{strings.ToUpper(f.Office)}
	}
	return filters, nil
}

// dateOrStar converts YYYY-MM-DD → Solr RFC3339 bound, or "*" if empty.
func dateOrStar(s string, start bool) string {
	if s == "" {
		return "*"
	}
	if len(s) == 10 {
		if start {
			return s + "T00:00:00Z"
		}
		return s + "T23:59:59Z"
	}
	return s
}

func newSwissregSearchCmd(use, short, target string) *cobra.Command {
	flags := &swissregSearchFlags{}
	c := &cobra.Command{
		Use:   use + " <query>",
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			filters, err := buildFilters(target, flags)
			if err != nil {
				return err
			}
			return runSwissregSearch(target, query, filters, flags.Max)
		},
	}
	addSearchFlags(c, flags)
	return c
}

var swissregGenericCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Raw search with explicit --target",
	Long:  "Valid --target values: " + strings.Join(api.SwissregTargets, ", "),
	Args:  cobra.MinimumNArgs(1),
}

var swissregDetailCmd = &cobra.Command{
	Use:   "detail <target> <id>",
	Short: "Look up a single record by internal id",
	Long: `Fetch a single record from swissreg by its internal URN id.

<target> is one of: ` + strings.Join(api.SwissregTargets, ", ") + `
<id> is either a bare internal id (e.g. 1206422825) or a full URN
(urn:ige:schutztitel:chmarke:1206422825).

Note: the internal id is not the trademark/patent number shown in the register;
it is the opaque id carried in search results under the ` + "`id`" + ` field.
Use ` + "`chli swissreg trademark <number>`" + ` first to discover it.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, id := args[0], args[1]
		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		r, err := client.SwissregDetail(target, id)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if r == nil {
			output.Error("no record found for " + target + " " + id)
			os.Exit(1)
		}
		output.Render(nil, nil, r)
		return nil
	},
}

var swissregImageCmd = &cobra.Command{
	Use:   "image <number-or-hash>",
	Short: "Show a trademark with its image as ASCII art",
	Long: `Render a trademark image to the terminal as ASCII art, together with
the trademark's metadata.

Behavior:
  (no flags)      Print metadata table + ASCII art (default)
  --out FILE      Save raw image bytes to FILE
  --url           Print only the image URL
  --raw           Write raw image bytes to stdout (for | imgcat, etc.)
  --cols N        ASCII art width in columns (default 60)

Accepts either a trademark number (e.g. 570105) or a 40-char SHA-1 image hash.
When a number is given, chli looks up the mark first; with a bare hash only
the ASCII art is shown (no metadata available).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		arg := args[0]
		out, _ := cmd.Flags().GetString("out")
		printURL, _ := cmd.Flags().GetBool("url")
		raw, _ := cmd.Flags().GetBool("raw")
		cols, _ := cmd.Flags().GetInt("cols")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		// Resolve input → (hash, optional trademark record).
		var hash string
		var record api.SwissregResult
		if len(arg) == 40 && isHex(arg) {
			hash = arg
		} else {
			resp, err := client.SwissregSearch(api.SwissregQuery{
				Target:       "chmarke",
				SearchString: arg,
				PageSize:     8,
			})
			if err != nil {
				output.Error(err.Error())
				os.Exit(1)
			}
			for _, r := range resp.Results {
				if h := r.First(api.SwissregImgScreen); h != "" {
					hash = h
					record = r
					break
				}
			}
			if hash == "" {
				output.Error("no image hash found for " + arg)
				os.Exit(1)
			}
		}

		if printURL {
			fmt.Println(api.SwissregImageURL(hash))
			return nil
		}

		data, ctype, err := client.SwissregFetchImage(hash)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if out != "" {
			if err := os.WriteFile(out, data, 0644); err != nil {
				output.Error(err.Error())
				os.Exit(1)
			}
			if output.IsInteractive() {
				fmt.Fprintf(os.Stderr, "Wrote %d bytes (%s) to %s\n", len(data), ctype, out)
			}
			return nil
		}

		if raw {
			_, err = os.Stdout.Write(data)
			return err
		}

		// Default: metadata table + ASCII art.
		if record != nil {
			printTrademarkDetail(record)
		}
		art, err := renderASCII(data, cols)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		fmt.Print(art)
		return nil
	},
}

// printTrademarkDetail writes a compact two-column info block for a trademark
// to stdout. Values come from the indexed search response.
func printTrademarkDetail(r api.SwissregResult) {
	rows := [][2]string{
		{"Number", r.First("markennummer_formatiert__type_string")},
		{"Mark", r.First("titel__type_text")},
		{"Type", shortEnum(r.First("schutztiteltyp__type_i18n"))},
		{"Status", shortEnum(r.First("schutztitelstatus__type_i18n"))},
		{"Classes", r.First("wdlklassennummernformatiert__type_string")},
		{"Owner", partyName(r.First("ra_inhaber__type_text_mv"))},
		{"Representative", partyName(r.First("ra_vertreter__type_text_mv"))},
		{"ID", r.First("id")},
		{"Image URL", api.SwissregImageURL(r.First(api.SwissregImgScreen))},
	}
	for _, row := range rows {
		if row[1] == "" {
			continue
		}
		fmt.Printf("%-14s  %s\n", row[0]+":", row[1])
	}
	fmt.Println()
}

func runSwissregSearch(target, query string, filters map[string][]string, max int) error {
	client, err := api.NewClient()
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
	client.NoCache = noCache
	client.Refresh = refresh

	resp, err := client.SwissregSearch(api.SwissregQuery{
		Target:       target,
		SearchString: query,
		Filters:      filters,
		PageSize:     max,
	})
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}

	headers, rows := swissregRows(target, resp.Results)
	output.Render(headers, rows, resp)
	if output.IsInteractive() && resp.TotalItems > len(resp.Results) {
		fmt.Fprintf(os.Stderr, "\nShowing %d of %d total matches. Use --max to see more (max %d).\n",
			len(resp.Results), resp.TotalItems, api.SwissregMaxPageSize)
	}
	return nil
}

func swissregRows(target string, results []api.SwissregResult) ([]string, [][]string) {
	switch target {
	case "patent", "publikationpatent":
		headers := []string{"Number", "Title", "Filed", "Status", "Owner"}
		rows := make([][]string, 0, len(results))
		for _, r := range results {
			rows = append(rows, []string{
				r.First("patentnummer_formatiert__type_string_ci"),
				output.Truncate(r.First("titel__type_text"), 50),
				r.First("anmeldedatum__type_date"),
				shortEnum(r.First("schutztitelstatus__type_i18n")),
				output.Truncate(partyName(r.First("ra_inhaber__type_text_mv")), 30),
			})
		}
		return headers, rows
	case "design", "publikationdesign":
		headers := []string{"Number", "Title", "Filed", "Status", "Owner"}
		rows := make([][]string, 0, len(results))
		for _, r := range results {
			num := r.First("designnummer_formatiert__type_string")
			if num == "" {
				num = r.First("gesuchsnummer__type_text_split_num")
			}
			title := r.First("bezeichnung__type_text")
			if title == "" {
				title = r.First("titel__type_text")
			}
			rows = append(rows, []string{
				num,
				output.Truncate(title, 50),
				r.First("anmeldedatum__type_date"),
				shortEnum(r.First("schutztitelstatus__type_i18n")),
				output.Truncate(partyName(r.First("ra_inhaber__type_text_mv")), 30),
			})
		}
		return headers, rows
	default: // chmarke (trademarks)
		headers := []string{"Number", "Mark", "Classes", "Status", "Type", "Owner", "Image"}
		rows := make([][]string, 0, len(results))
		for _, r := range results {
			rows = append(rows, []string{
				r.First("markennummer_formatiert__type_string"),
				output.Truncate(r.First("titel__type_text"), 40),
				r.First("wdlklassennummernformatiert__type_string"),
				shortEnum(r.First("schutztitelstatus__type_i18n")),
				shortEnum(r.First("schutztiteltyp__type_i18n")),
				output.Truncate(partyName(r.First("ra_inhaber__type_text_mv")), 30),
				api.SwissregImageURL(r.First(api.SwissregImgScreen)),
			})
		}
		return headers, rows
	}
}

// shortEnum converts "schutztitel.enum.schutztitelstatus.aktiv" → "aktiv".
func shortEnum(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// partyName extracts the name from a pipe-delimited party string:
//
//	"1184717|RMT Reinhardt Microtech AG|…|7323 Wangs|CH"
func partyName(s string) string {
	parts := strings.Split(s, "|")
	if len(parts) >= 2 {
		return parts[1]
	}
	return s
}

func isHex(s string) bool {
	for _, r := range s {
		if !(('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')) {
			return false
		}
	}
	return true
}

func init() {
	// Shortcut subcommands (one per target).
	for _, def := range []struct{ use, short, target string }{
		{"trademark", "Search Swiss and international trademarks", "chmarke"},
		{"patent", "Search Swiss and European patents", "patent"},
		{"design", "Search Swiss design registrations", "design"},
		{"patent-pub", "Search patent publications", "publikationpatent"},
		{"design-pub", "Search design publications", "publikationdesign"},
	} {
		swissregCmd.AddCommand(newSwissregSearchCmd(def.use, def.short, def.target))
	}

	// Generic search with --target.
	genericFlags := &swissregSearchFlags{}
	swissregGenericCmd.Flags().String("target", "", "Search target (chmarke, patent, design, publikationpatent, publikationdesign)")
	addSearchFlags(swissregGenericCmd, genericFlags)
	swissregGenericCmd.RunE = func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetString("target")
		if target == "" {
			return fmt.Errorf("--target is required (one of: %s)", strings.Join(api.SwissregTargets, ", "))
		}
		query := strings.Join(args, " ")
		filters, err := buildFilters(target, genericFlags)
		if err != nil {
			return err
		}
		return runSwissregSearch(target, query, filters, genericFlags.Max)
	}
	swissregCmd.AddCommand(swissregGenericCmd)

	// Image download.
	swissregImageCmd.Flags().String("out", "", "Save raw image bytes to FILE")
	swissregImageCmd.Flags().Bool("url", false, "Print the image URL only")
	swissregImageCmd.Flags().Bool("raw", false, "Write raw image bytes to stdout")
	swissregImageCmd.Flags().Int("cols", 60, "ASCII art width in columns")
	swissregCmd.AddCommand(swissregImageCmd)

	// Detail lookup.
	swissregCmd.AddCommand(swissregDetailCmd)

	rootCmd.AddCommand(swissregCmd)
}
