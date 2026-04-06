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

	fedlexCmd.AddCommand(fedlexSRCmd)
	fedlexCmd.AddCommand(fedlexSearchCmd)
	fedlexCmd.AddCommand(fedlexSPARQLCmd)
	fedlexCmd.AddCommand(fedlexBBLCmd)
	fedlexCmd.AddCommand(fedlexConsultationCmd)
	fedlexCmd.AddCommand(fedlexTreatyCmd)
	fedlexCmd.AddCommand(fedlexFetchCmd)

	rootCmd.AddCommand(fedlexCmd)
}
