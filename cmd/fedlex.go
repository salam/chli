package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var fedlexCmd = &cobra.Command{
	Use:   "fedlex",
	Short: "Access Swiss federal law (Fedlex / SR)",
	Long:  "Query the Fedlex SPARQL endpoint for Swiss federal legislation, Federal Gazette entries, consultations, and treaties.",
}

var fedlexSRCmd = &cobra.Command{
	Use:   "sr <number>",
	Short: "Look up a law by SR number",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFedlexClient()
		if err != nil {
			return err
		}
		entries, err := client.FedlexSR(args[0], output.Lang)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(entries) == 0 {
			output.Error("no results found for SR " + args[0])
			os.Exit(1)
		}
		if output.IsInteractive() {
			output.Section("SR " + args[0])
			headers := []string{"URI", "Title", "Date", "Status"}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{
					e.URI,
					output.Truncate(e.Title, 60),
					e.DateDoc,
					e.InForceLabel,
				})
			}
			output.Table(headers, rows)
		} else {
			output.JSON(entries)
		}
		return nil
	},
}

var fedlexSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search federal law by title",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFedlexClient()
		if err != nil {
			return err
		}
		results, err := client.FedlexSearch(args[0], output.Lang)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(results) == 0 {
			output.Error("no results found for query: " + args[0])
			os.Exit(1)
		}
		if output.IsInteractive() {
			output.Section("Search: " + args[0])
			headers := []string{"Identifier", "Title", "Date", "In Force"}
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				rows = append(rows, []string{
					r.Identifier,
					output.Truncate(r.Title, 60),
					r.DateDoc,
					r.InForce,
				})
			}
			output.Table(headers, rows)
		} else {
			output.JSON(results)
		}
		return nil
	},
}

var fedlexSPARQLCmd = &cobra.Command{
	Use:   "sparql <query-or-@file>",
	Short: "Execute a raw SPARQL query",
	Long:  "Execute an arbitrary SPARQL query against the Fedlex endpoint. Pass the query directly or use @filename to read from a file.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFedlexClient()
		if err != nil {
			return err
		}
		query := args[0]
		if strings.HasPrefix(query, "@") {
			data, err := os.ReadFile(query[1:])
			if err != nil {
				output.Error(fmt.Sprintf("reading query file: %s", err))
				os.Exit(1)
			}
			query = string(data)
		}
		result, err := client.FedlexSPARQL(query)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		output.JSON(result)
		return nil
	},
}

var (
	bblYear string
)

var fedlexBBLCmd = &cobra.Command{
	Use:   "bbl",
	Short: "Federal Gazette (Bundesblatt) entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFedlexClient()
		if err != nil {
			return err
		}
		if bblYear == "" {
			bblYear = "2025"
		}
		entries, err := client.FedlexBBL(bblYear, output.Lang)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(entries) == 0 {
			output.Error("no Federal Gazette entries found for year " + bblYear)
			os.Exit(1)
		}
		if output.IsInteractive() {
			output.Section("Federal Gazette " + bblYear)
			headers := []string{"Title", "Date"}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{
					output.Truncate(e.Title, 70),
					e.DateDoc,
				})
			}
			output.Table(headers, rows)
		} else {
			output.JSON(entries)
		}
		return nil
	},
}

var (
	consultationStatus string
)

var fedlexConsultationCmd = &cobra.Command{
	Use:   "consultation",
	Short: "Search consultations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFedlexClient()
		if err != nil {
			return err
		}
		entries, err := client.FedlexConsultations(consultationStatus, output.Lang)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(entries) == 0 {
			output.Error("no consultations found")
			os.Exit(1)
		}
		if output.IsInteractive() {
			output.Section("Consultations")
			headers := []string{"Title", "Date", "Status"}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{
					output.Truncate(e.Title, 60),
					e.DateDoc,
					e.Status,
				})
			}
			output.Table(headers, rows)
		} else {
			output.JSON(entries)
		}
		return nil
	},
}

var (
	treatyPartner string
	treatyYear    string
)

var fedlexTreatyCmd = &cobra.Command{
	Use:   "treaty",
	Short: "Search international treaties",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFedlexClient()
		if err != nil {
			return err
		}
		entries, err := client.FedlexTreaties(treatyPartner, treatyYear, output.Lang)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(entries) == 0 {
			output.Error("no treaties found")
			os.Exit(1)
		}
		if output.IsInteractive() {
			output.Section("Treaties")
			headers := []string{"Title", "Date", "Partner"}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{
					output.Truncate(e.Title, 60),
					e.DateDoc,
					e.Partner,
				})
			}
			output.Table(headers, rows)
		} else {
			output.JSON(entries)
		}
		return nil
	},
}

var (
	diffV1 string
	diffV2 string
)

var fedlexDiffCmd = &cobra.Command{
	Use:   "diff <sr-number>",
	Short: "Compare two versions of the same SR law text",
	Long:  "Fetch available versions of a law by SR number and show a line-by-line diff of their titles. Use --v1 and --v2 to specify dates.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newFedlexClient()
		if err != nil {
			return err
		}
		srNumber := args[0]
		versions, err := client.FedlexVersions(srNumber, output.Lang)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(versions) == 0 {
			output.Error("no versions found for SR " + srNumber)
			os.Exit(1)
		}

		// If no versions specified, list available versions
		if diffV1 == "" || diffV2 == "" {
			if output.IsInteractive() {
				output.Section("Available versions for SR " + srNumber)
				headers := []string{"Date", "Title", "URI"}
				rows := make([][]string, 0, len(versions))
				for _, v := range versions {
					rows = append(rows, []string{
						v.Date,
						output.Truncate(v.Title, 60),
						v.URI,
					})
				}
				output.Table(headers, rows)
				fmt.Println()
				fmt.Println("Use --v1 DATE --v2 DATE to compare two versions.")
			} else {
				output.JSON(versions)
			}
			return nil
		}

		// Find versions matching the specified dates
		var v1, v2 *api.FedlexVersion
		for i := range versions {
			if versions[i].Date == diffV1 {
				v1 = &versions[i]
			}
			if versions[i].Date == diffV2 {
				v2 = &versions[i]
			}
		}
		if v1 == nil {
			output.Error("version not found for date: " + diffV1)
			os.Exit(1)
		}
		if v2 == nil {
			output.Error("version not found for date: " + diffV2)
			os.Exit(1)
		}

		// Show diff of titles
		if output.IsInteractive() {
			output.Section("Diff SR " + srNumber + ": " + diffV1 + " vs " + diffV2)
			lines1 := strings.Split(v1.Title, "\n")
			lines2 := strings.Split(v2.Title, "\n")
			diffLines := lineDiff(lines1, lines2)
			for _, dl := range diffLines {
				fmt.Println(dl)
			}
		} else {
			output.JSON(map[string]any{
				"sr":    srNumber,
				"v1":    v1,
				"v2":    v2,
				"diff":  lineDiff(strings.Split(v1.Title, "\n"), strings.Split(v2.Title, "\n")),
			})
		}
		return nil
	},
}

// lineDiff produces a simple line-by-line comparison between two sets of lines.
// Lines only in a are prefixed with "- ", lines only in b with "+ ", common lines with "  ".
func lineDiff(a, b []string) []string {
	// Build a simple LCS-based diff
	n, m := len(a), len(b)
	// LCS table
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var result []string
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			result = append(result, "  "+a[i])
			i++
			j++
		} else if lcs[i+1][j] >= lcs[i][j+1] {
			result = append(result, "- "+a[i])
			i++
		} else {
			result = append(result, "+ "+b[j])
			j++
		}
	}
	for ; i < n; i++ {
		result = append(result, "- "+a[i])
	}
	for ; j < m; j++ {
		result = append(result, "+ "+b[j])
	}
	return result
}

var (
	fetchFormat string
)

var fedlexFetchCmd = &cobra.Command{
	Use:   "fetch <eli-uri>",
	Short: "Download or display a law manifestation",
	Long:  "Given an ELI URI, construct the download URL. Currently prints the URL for the requested format.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uri := args[0]
		langCode := api.LangURI(output.Lang)
		if fetchFormat == "" {
			fetchFormat = "html"
		}
		// Construct the manifestation URL from the ELI URI.
		// Pattern: <eli-uri>/<lang>/<format>
		downloadURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(uri, "/"), strings.ToLower(langCode), fetchFormat)
		if output.IsInteractive() {
			output.Section("Fetch")
			fmt.Printf("URI:    %s\n", uri)
			fmt.Printf("Format: %s\n", fetchFormat)
			fmt.Printf("URL:    %s\n", downloadURL)
		} else {
			output.JSON(map[string]string{
				"uri":    uri,
				"format": fetchFormat,
				"url":    downloadURL,
			})
		}
		return nil
	},
}

func newFedlexClient() (*api.Client, error) {
	client, err := api.NewClient()
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
	client.NoCache = noCache
	client.Refresh = refresh
	return client, nil
}

func init() {
	fedlexBBLCmd.Flags().StringVar(&bblYear, "year", "", "Filter by year (e.g. 2025)")
	fedlexConsultationCmd.Flags().StringVar(&consultationStatus, "status", "", "Filter by status")
	fedlexTreatyCmd.Flags().StringVar(&treatyPartner, "partner", "", "Filter by treaty partner")
	fedlexTreatyCmd.Flags().StringVar(&treatyYear, "year", "", "Filter by year")
	fedlexFetchCmd.Flags().StringVar(&fetchFormat, "format", "html", "Manifestation format (html, pdf, xml)")
	fedlexDiffCmd.Flags().StringVar(&diffV1, "v1", "", "Date of first version (e.g. 2020-01-01)")
	fedlexDiffCmd.Flags().StringVar(&diffV2, "v2", "", "Date of second version (e.g. 2024-01-01)")

	fedlexCmd.AddCommand(fedlexSRCmd)
	fedlexCmd.AddCommand(fedlexSearchCmd)
	fedlexCmd.AddCommand(fedlexSPARQLCmd)
	fedlexCmd.AddCommand(fedlexBBLCmd)
	fedlexCmd.AddCommand(fedlexConsultationCmd)
	fedlexCmd.AddCommand(fedlexTreatyCmd)
	fedlexCmd.AddCommand(fedlexFetchCmd)
	fedlexCmd.AddCommand(fedlexDiffCmd)

	rootCmd.AddCommand(fedlexCmd)
}
