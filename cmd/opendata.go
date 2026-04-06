package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var opendataCmd = &cobra.Command{
	Use:   "opendata",
	Short: "Query opendata.swiss (CKAN)",
	Long:  "Search datasets, view dataset details, and list organizations on opendata.swiss.",
}

var opendataSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search datasets on opendata.swiss",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		org, _ := cmd.Flags().GetString("org")
		format, _ := cmd.Flags().GetString("format")
		rows, _ := cmd.Flags().GetInt("rows")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		result, err := client.OpendataSearch(query, org, format, rows, 0)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			fmt.Printf("Found %d datasets (showing %d)\n\n", result.Count, len(result.Results))
			headers := []string{"Name", "Title", "Organization", "Resources", "Modified"}
			var rows [][]string
			for _, ds := range result.Results {
				rows = append(rows, []string{
					ds.Name,
					output.Truncate(ds.Title.Pick(output.Lang), 50),
					ds.Organization.Title.Pick(output.Lang),
					strconv.Itoa(ds.NumResources),
					ds.MetadataModified,
				})
			}
			output.Table(headers, rows)
		} else {
			output.JSON(result)
		}
		return nil
	},
}

var opendataDatasetCmd = &cobra.Command{
	Use:   "dataset <id>",
	Short: "Show dataset details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		ds, err := client.OpendataDataset(id)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			lang := output.Lang
			fmt.Printf("Name:         %s\n", ds.Name)
			fmt.Printf("Title:        %s\n", ds.Title.Pick(lang))
			fmt.Printf("Organization: %s\n", ds.Organization.Title.Pick(lang))
			fmt.Printf("Issued:       %s\n", ds.Issued)
			fmt.Printf("Modified:     %s\n", ds.MetadataModified)
			fmt.Printf("Description:  %s\n", output.Truncate(ds.Description.Pick(lang), 200))

			if len(ds.Resources) > 0 {
				output.Section("Resources")
				headers := []string{"Format", "Name", "URL"}
				var rows [][]string
				for _, r := range ds.Resources {
					rows = append(rows, []string{
						r.Format,
						output.Truncate(r.Name.Pick(lang), 60),
						r.DownloadURL,
					})
				}
				output.Table(headers, rows)
			}
		} else {
			output.JSON(ds)
		}
		return nil
	},
}

var opendataOrgsCmd = &cobra.Command{
	Use:   "orgs",
	Short: "List organizations on opendata.swiss",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		orgs, err := client.OpendataOrgs()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			fmt.Printf("Found %d organizations\n\n", len(orgs))
			headers := []string{"Name", "Title"}
			var rows [][]string
			for _, o := range orgs {
				rows = append(rows, []string{
					o.Name,
					output.Truncate(o.Title.Pick(output.Lang), 60),
				})
			}
			output.Table(headers, rows)
		} else {
			output.JSON(orgs)
		}
		return nil
	},
}

func init() {
	opendataSearchCmd.Flags().String("org", "", "Filter by organization slug")
	opendataSearchCmd.Flags().String("format", "", "Filter by resource format (e.g. CSV, JSON)")
	opendataSearchCmd.Flags().Int("rows", 20, "Number of results to return")

	opendataCmd.AddCommand(opendataSearchCmd)
	opendataCmd.AddCommand(opendataDatasetCmd)
	opendataCmd.AddCommand(opendataOrgsCmd)

	rootCmd.AddCommand(opendataCmd)
}
