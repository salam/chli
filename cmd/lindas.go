package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var lindasCmd = &cobra.Command{
	Use:   "lindas",
	Short: "LINDAS linked-data SPARQL endpoint",
	Long: `Query LINDAS (lindas.admin.ch), the Swiss federal linked-data hub that
exposes datasets from IPI (trademarks/patents), BFS statistics, public
procurement, energy, and more — one SPARQL endpoint, many datasets.`,
}

var lindasSPARQLCmd = &cobra.Command{
	Use:   "sparql <query-or-@file>",
	Short: "Execute a raw SPARQL query against LINDAS",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		query := args[0]
		if strings.HasPrefix(query, "@") {
			data, err := os.ReadFile(query[1:])
			if err != nil {
				output.Error(fmt.Sprintf("reading query file: %s", err))
				os.Exit(1)
			}
			query = string(data)
		}

		result, err := client.LindasSPARQL(query)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		headers, rows := sparqlRows(result)
		output.Render(headers, rows, result)
		return nil
	},
}

var lindasDatasetsCmd = &cobra.Command{
	Use:   "datasets",
	Short: "List datasets exposed by LINDAS",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		result, err := client.LindasDatasets()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		headers, rows := sparqlRows(result)
		output.Render(headers, rows, result)
		return nil
	},
}

// sparqlRows converts a SPARQLResult into table rows in the order declared
// by result.Head.Vars.
func sparqlRows(r *api.SPARQLResult) ([]string, [][]string) {
	if r == nil {
		return nil, nil
	}
	headers := r.Head.Vars
	rows := make([][]string, 0, len(r.Results.Bindings))
	for _, b := range r.Results.Bindings {
		row := make([]string, len(headers))
		for i, v := range headers {
			if cell, ok := b[v]; ok {
				row[i] = output.Truncate(cell.Value, 80)
			}
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func init() {
	lindasCmd.AddCommand(lindasSPARQLCmd)
	lindasCmd.AddCommand(lindasDatasetsCmd)
	rootCmd.AddCommand(lindasCmd)
}
