package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var entscheidCmd = &cobra.Command{
	Use:   "entscheid",
	Short: "Swiss court decisions (entscheidsuche.ch)",
	Long:  "Search and view Swiss court decisions from entscheidsuche.ch, covering federal and cantonal courts.",
}

var entscheidSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search court decisions",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		court, _ := cmd.Flags().GetString("court")
		dateFrom, _ := cmd.Flags().GetString("from")
		dateTo, _ := cmd.Flags().GetString("to")
		size, _ := cmd.Flags().GetInt("size")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		result, err := client.EntscheidSearch(query, court, dateFrom, dateTo, size)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(result)
			return nil
		}

		total := result.Hits.Total.Value
		rel := ""
		if result.Hits.Total.Relation == "gte" {
			rel = "+"
		}
		fmt.Printf("Found %d%s decisions (showing %d)\n\n", total, rel, len(result.Hits.Hits))

		if len(result.Hits.Hits) == 0 {
			fmt.Println("No results.")
			return nil
		}

		headers := []string{"Date", "Canton", "Title", "Reference"}
		rows := make([][]string, 0, len(result.Hits.Hits))
		for _, hit := range result.Hits.Hits {
			d := hit.Source
			title := output.Truncate(d.Title.Pick(output.Lang), 50)
			ref := ""
			if len(d.Reference) > 0 {
				ref = output.Truncate(strings.Join(d.Reference, "; "), 40)
			}
			rows = append(rows, []string{
				d.Date,
				d.Canton,
				title,
				ref,
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var entscheidGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Fetch a single court decision by ID",
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

		decision, err := client.EntscheidGet(id)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(decision)
			return nil
		}

		output.Section("Decision")
		fmt.Printf("ID:      %s\n", decision.ID)
		fmt.Printf("Date:    %s\n", decision.Date)
		fmt.Printf("Canton:  %s\n", decision.Canton)
		fmt.Printf("Title:   %s\n", decision.Title.Pick(output.Lang))

		if len(decision.Reference) > 0 {
			fmt.Printf("Ref:     %s\n", strings.Join(decision.Reference, "; "))
		}

		abstract := decision.Abstract.Pick(output.Lang)
		if abstract != "" {
			output.Section("Abstract")
			fmt.Println(abstract)
		}

		if decision.Attachment.ContentURL != "" {
			output.Section("Document")
			fmt.Printf("URL:   %s\n", decision.Attachment.ContentURL)
			fmt.Printf("Type:  %s\n", decision.Attachment.ContentType)
		}

		return nil
	},
}

var entscheidCourtsCmd = &cobra.Command{
	Use:   "courts",
	Short: "List known courts and cantons",
	RunE: func(cmd *cobra.Command, args []string) error {
		courts := api.EntscheidCourts()

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(courts)
			return nil
		}

		headers := []string{"Code", "Name"}
		rows := make([][]string, 0, len(courts))
		for _, c := range courts {
			rows = append(rows, []string{c.Code, c.Name})
		}
		output.Table(headers, rows)
		return nil
	},
}

func init() {
	entscheidSearchCmd.Flags().String("court", "", "Filter by court or canton (e.g. BGer, BVGer, ZH)")
	entscheidSearchCmd.Flags().String("from", "", "Filter decisions from date (YYYY-MM-DD)")
	entscheidSearchCmd.Flags().String("to", "", "Filter decisions to date (YYYY-MM-DD)")
	entscheidSearchCmd.Flags().Int("size", 10, "Number of results to return")

	entscheidCmd.AddCommand(entscheidSearchCmd)
	entscheidCmd.AddCommand(entscheidGetCmd)
	entscheidCmd.AddCommand(entscheidCourtsCmd)
	rootCmd.AddCommand(entscheidCmd)
}
