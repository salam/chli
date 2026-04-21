package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

// zefixAuthBinding is also used by `chli uid` subcommands, since UID
// lookups are served via the Zefix REST API and share the same credential.
var zefixAuthBinding = authBinding{
	Service: "zefix",
	EnvUser: "ZEFIX_USER",
	EnvPass: "ZEFIX_PASS",
	HelpLong: `Store credentials for the Zefix public REST API.

Credentials are optional: without them, zefix/uid commands fall back to the
unauthenticated endpoints used by the zefix.ch website. Register at
https://www.zefix.admin.ch/ (API section) for the official API if you want a
versioned, documented contract. Stored credentials are written to
~/.config/chli/credentials.json with 0600 permissions.

Precedence when a command needs credentials:
  1. Environment variables (ZEFIX_USER / ZEFIX_PASS)
  2. Stored credentials (this command)
  3. Public fallback (no credentials required)

Note: ` + "`chli uid login`" + ` and ` + "`chli zefix login`" + ` share the same entry,
since UID lookup goes through the Zefix REST API.`,
}

var zefixCmd = &cobra.Command{
	Use:   "zefix",
	Short: "Swiss commercial register (Zefix)",
	Long: `Search and look up Swiss companies via the Zefix REST API. Uses the authenticated official endpoint when credentials are configured and falls back to the unauthenticated zefix.ch endpoints otherwise.`,
}

var zefixSearchCmd = &cobra.Command{
	Use:   "search <name>",
	Short: "Search the commercial register by company name",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, " ")
		canton, _ := cmd.Flags().GetString("canton")
		max, _ := cmd.Flags().GetInt("max")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		results, err := client.ZefixSearch(name, canton, output.Lang, max)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		headers, rows := zefixRows(results)
		output.Render(headers, rows, results)
		zefixPrintFollowUp(results)
		return nil
	},
}

var zefixCompanyCmd = &cobra.Command{
	Use:   "company <uid-or-chid>",
	Short: "Look up a company by UID (CHE-…) or CHID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		id := args[0]
		var results []api.ZefixCompany
		if looksLikeUID(id) {
			results, err = client.ZefixCompanyByUID(id)
		} else {
			results, err = client.ZefixCompanyByCHID(id)
		}
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(results) == 0 {
			output.Error("no company found for " + id)
			os.Exit(1)
		}

		headers, rows := zefixRows(results)
		output.Render(headers, rows, results)
		zefixPrintFollowUp(results)
		return nil
	},
}

// zefixPrintFollowUp nudges the user toward SHAB for the first hit. Shown
// only on interactive, table-style output — machine-readable formats stay
// clean.
func zefixPrintFollowUp(results []api.ZefixCompany) {
	if len(results) == 0 || !output.IsInteractive() {
		return
	}
	uid := results[0].UIDFormatted
	if uid == "" {
		uid = api.FormatUID(results[0].UID)
	}
	if uid == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "\nFor SHAB publications on this entity:\n  chli shab search %s\nThen walk the FOSC chain from a hit:\n  chli shab history <publication-number>\n", uid)
}

func zefixRows(results []api.ZefixCompany) ([]string, [][]string) {
	headers := []string{"UID", "Name", "Seat", "Canton", "Status", "Legal Form"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		uid := r.UIDFormatted
		if uid == "" {
			uid = api.FormatUID(r.UID)
		}
		lf := ""
		if r.LegalForm != nil {
			lf = r.LegalForm.ShortName.Pick(output.Lang)
			if lf == "" {
				lf = r.LegalForm.Name.Pick(output.Lang)
			}
		}
		rows = append(rows, []string{
			uid,
			output.Truncate(r.Name, 50),
			r.LegalSeat,
			r.Canton,
			r.Status,
			lf,
		})
	}
	return headers, rows
}

func looksLikeUID(s string) bool {
	up := strings.ToUpper(s)
	return strings.HasPrefix(up, "CHE") || strings.HasPrefix(up, "CHE-")
}

func init() {
	zefixSearchCmd.Flags().String("canton", "", "Filter by canton code (e.g. ZH, BE)")
	zefixSearchCmd.Flags().Int("max", 30, "Maximum results (1-1000)")

	zefixCmd.AddCommand(zefixSearchCmd)
	zefixCmd.AddCommand(zefixCompanyCmd)
	zefixCmd.AddCommand(newLoginCmd(zefixAuthBinding))
	zefixCmd.AddCommand(newLogoutCmd(zefixAuthBinding))
	zefixCmd.AddCommand(newStatusCmd(zefixAuthBinding))
	rootCmd.AddCommand(zefixCmd)
}
