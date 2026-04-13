package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

// changeSummary produces a short label for a publication based on its
// transaction kind, falling back to its title (truncated to 40 chars).
func changeSummary(pub *api.SHABPublicationXML) string {
	if pub == nil {
		return ""
	}
	if tx := pub.Content.Transaction; tx != nil {
		switch {
		case tx.Creation != nil:
			return "Neueintragung"
		case tx.Deletion != nil:
			return "Löschung"
		case tx.Update != nil:
			labels := tx.Update.Changements.ChangedLabels()
			if len(labels) == 0 {
				return "Mutation"
			}
			return "Mutation — " + strings.Join(labels, ", ")
		}
	}
	if pub.Meta.Title != nil {
		return output.Truncate(pub.Meta.Title.Pick(output.Lang), 40)
	}
	return ""
}

// historyEntry is one row of the timeline.
type historyEntry struct {
	Date              string `json:"date"`
	PublicationNumber string `json:"publicationNumber"`
	URL               string `json:"url"`
	ChangeSummary     string `json:"changeSummary"`
	IsCurrent         bool   `json:"isCurrent"`
}

var shabHistoryCmd = &cobra.Command{
	Use:   "history <number|uuid>",
	Short: "Show the chain of FOSC publications for the same legal entity",
	Long:  "Walks the lastFosc back-pointers from the given publication until there are no more prior FOSC entries.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		depth, _ := cmd.Flags().GetInt("depth")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		id, err := client.SHABResolveID(args[0])
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		chain, err := client.SHABHistory(id, depth)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		if len(chain) == 0 {
			fmt.Println("No publication found.")
			return nil
		}

		// chain is newest -> oldest; reverse for display
		entries := make([]historyEntry, 0, len(chain))
		for i := len(chain) - 1; i >= 0; i-- {
			pub := chain[i]
			entries = append(entries, historyEntry{
				Date:              pub.Meta.PublicationDate,
				PublicationNumber: pub.Meta.PublicationNumber,
				URL:               api.SHABPublicationURL(pub.Meta.ID),
				ChangeSummary:     changeSummary(pub),
				IsCurrent:         i == 0,
			})
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(entries)
			return nil
		}

		for _, e := range entries {
			marker := ""
			if e.IsCurrent {
				marker = "  ← current"
			}
			date := e.Date
			if len(date) >= 10 {
				date = date[:10]
			}
			fmt.Printf("%-10s  %-20s  %-40s  %s%s\n",
				date,
				e.PublicationNumber,
				output.Truncate(e.ChangeSummary, 40),
				output.Hyperlink(e.URL, "link"),
				marker,
			)
		}
		if len(chain) == 1 && (chain[0].Content.LastFosc == nil || chain[0].Content.LastFosc.Sequence == "") {
			fmt.Println("\nNo prior FOSC entries referenced by this publication.")
		}
		return nil
	},
}

func init() {
	shabHistoryCmd.Flags().Int("depth", 0, "Maximum number of back-hops (0 = unlimited)")
	shabCmd.AddCommand(shabHistoryCmd)
}
