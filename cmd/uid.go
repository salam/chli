package cmd

import (
	"os"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

// The BFS UID register proper is a SOAP service (UidPublicServices.svc) with
// a free-but-registered account. The openly queryable surface for the same
// entity data is Zefix. `chli uid` normalises user input and routes to the
// Zefix REST endpoint so callers have a UID-centric entry point.

var uidCmd = &cobra.Command{
	Use:   "uid",
	Short: "UID (Unternehmens-Identifikationsnummer) lookup",
	Long: `Look up and format Swiss business identifier numbers (UID).

Data source: Zefix public REST API (the openly queryable surface for UID-holders
that are in the commercial register). The BFS UID register SOAP service
(uid.admin.ch) is authenticated and not wrapped here.`,
}

var uidLookupCmd = &cobra.Command{
	Use:   "lookup <uid>",
	Short: "Look up an entity by UID (CHE-…)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		results, err := client.ZefixCompanyByUID(args[0])
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(results) == 0 {
			output.Error("no entry found for UID " + api.FormatUID(args[0]))
			os.Exit(1)
		}
		headers, rows := zefixRows(results)
		output.Render(headers, rows, results)
		return nil
	},
}

var uidFormatCmd = &cobra.Command{
	Use:   "format <input>",
	Short: "Normalize / pretty-format a UID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw := args[0]
		canonical := api.NormalizeUID(raw)
		pretty := api.FormatUID(raw)
		data := map[string]string{
			"input":     raw,
			"canonical": canonical,
			"formatted": pretty,
		}
		headers := []string{"Input", "Canonical", "Formatted"}
		rows := [][]string{{raw, canonical, pretty}}
		output.Render(headers, rows, data)
		return nil
	},
}

func init() {
	uidCmd.AddCommand(uidLookupCmd)
	uidCmd.AddCommand(uidFormatCmd)
	// UID lookups go through Zefix — the login/logout/status commands share
	// the "zefix" credentials entry, so `chli uid login` and `chli zefix login`
	// are interchangeable.
	uidCmd.AddCommand(newLoginCmd(zefixAuthBinding))
	uidCmd.AddCommand(newLogoutCmd(zefixAuthBinding))
	uidCmd.AddCommand(newStatusCmd(zefixAuthBinding))
	rootCmd.AddCommand(uidCmd)
}
