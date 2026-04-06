package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var feedCmd = &cobra.Command{
	Use:   "feed",
	Short: "Poll for new items and print them as they appear",
	Long:  "Watch for new publications, court decisions, or parliament business in real-time.",
}

// --- feed shab ---

var feedShabCmd = &cobra.Command{
	Use:   "shab",
	Short: "Poll SHAB for new publications",
	RunE: func(cmd *cobra.Command, args []string) error {
		rubricFlag, _ := cmd.Flags().GetString("rubric")
		interval, _ := cmd.Flags().GetInt("interval")

		var rubrics []string
		if rubricFlag != "" {
			rubrics = strings.Split(rubricFlag, ",")
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		seen := make(map[string]bool)
		first := true

		return pollLoop(ctx, interval, func() error {
			client, err := api.NewClient()
			if err != nil {
				return err
			}
			client.NoCache = true

			result, err := client.SHABSearch("*", rubrics, 0, 20)
			if err != nil {
				output.Verbosef("SHAB fetch error: %s", err)
				return nil // don't abort on transient errors
			}

			for _, pub := range result.Content {
				id := pub.Meta.PublicationNumber
				if id == "" {
					continue
				}
				if seen[id] {
					continue
				}
				seen[id] = true
				if first {
					continue // skip initial batch, only show new items
				}

				m := pub.Meta
				title := output.Truncate(m.Title.Pick(output.Lang), 60)
				date := ""
				if len(m.PublicationDate) >= 10 {
					date = m.PublicationDate[:10]
				}

				if output.ForceJSON || !output.IsInteractive() {
					output.JSON(map[string]string{
						"number": m.PublicationNumber,
						"rubric": m.Rubric,
						"title":  m.Title.Pick(output.Lang),
						"date":   date,
					})
				} else {
					fmt.Printf("[%s] %s  %s  %s\n",
						time.Now().Format("15:04:05"),
						m.PublicationNumber,
						m.Rubric,
						title,
					)
				}
			}
			first = false
			return nil
		})
	},
}

// --- feed entscheid ---

var feedEntscheidCmd = &cobra.Command{
	Use:   "entscheid",
	Short: "Poll for new court decisions",
	RunE: func(cmd *cobra.Command, args []string) error {
		court, _ := cmd.Flags().GetString("court")
		interval, _ := cmd.Flags().GetInt("interval")

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		seen := make(map[string]bool)
		first := true

		return pollLoop(ctx, interval, func() error {
			client, err := api.NewClient()
			if err != nil {
				return err
			}
			client.NoCache = true

			result, err := client.EntscheidSearch("*", court, "", "", 20)
			if err != nil {
				output.Verbosef("Entscheid fetch error: %s", err)
				return nil
			}

			for _, hit := range result.Hits.Hits {
				id := hit.ID
				if id == "" {
					continue
				}
				if seen[id] {
					continue
				}
				seen[id] = true
				if first {
					continue
				}

				d := hit.Source
				title := output.Truncate(d.Title.Pick(output.Lang), 50)

				if output.ForceJSON || !output.IsInteractive() {
					output.JSON(map[string]string{
						"id":     id,
						"date":   d.Date,
						"canton": d.Canton,
						"title":  d.Title.Pick(output.Lang),
					})
				} else {
					fmt.Printf("[%s] %s  %s  %s  %s\n",
						time.Now().Format("15:04:05"),
						d.Date,
						d.Canton,
						title,
						id,
					)
				}
			}
			first = false
			return nil
		})
	},
}

// --- feed parl ---

var feedParlCmd = &cobra.Command{
	Use:   "parl",
	Short: "Poll for new parliament business",
	RunE: func(cmd *cobra.Command, args []string) error {
		typeFilter, _ := cmd.Flags().GetString("type")
		interval, _ := cmd.Flags().GetInt("interval")

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		seen := make(map[string]bool)
		first := true

		return pollLoop(ctx, interval, func() error {
			client, err := newParlClient()
			if err != nil {
				return err
			}
			client.NoCache = true

			// Fetch recent business ordered by submission date descending
			filter := ""
			if typeFilter != "" {
				filter = fmt.Sprintf("BusinessTypeAbbreviation eq '%s'", typeFilter)
			}
			selectFields := "BusinessShortNumber,Title,BusinessTypeAbbreviation,BusinessStatusText,SubmissionDate"
			path := fmt.Sprintf("/odata.svc/Business?$orderby=SubmissionDate desc&$top=20&$select=%s", selectFields)
			if filter != "" {
				path += "&$filter=" + filter
			}

			var result struct {
				D struct {
					Results []struct {
						BusinessShortNumber      *string `json:"BusinessShortNumber"`
						Title                    *string `json:"Title"`
						BusinessTypeAbbreviation *string `json:"BusinessTypeAbbreviation"`
						BusinessStatusText       *string `json:"BusinessStatusText"`
						SubmissionDate           *string `json:"SubmissionDate"`
					} `json:"results"`
				} `json:"d"`
			}
			if err := client.DoJSON("https://ws.parlament.ch", path, &result); err != nil {
				output.Verbosef("Parliament fetch error: %s", err)
				return nil
			}

			for _, b := range result.D.Results {
				id := api.Str(b.BusinessShortNumber)
				if id == "" {
					continue
				}
				if seen[id] {
					continue
				}
				seen[id] = true
				if first {
					continue
				}

				title := output.Truncate(api.Str(b.Title), 50)
				typ := api.Str(b.BusinessTypeAbbreviation)
				status := api.Str(b.BusinessStatusText)
				date := api.ParseODataDate(api.Str(b.SubmissionDate))

				if output.ForceJSON || !output.IsInteractive() {
					output.JSON(map[string]string{
						"number": id,
						"type":   typ,
						"title":  api.Str(b.Title),
						"status": status,
						"date":   date,
					})
				} else {
					fmt.Printf("[%s] %s  %s  %s  %s  %s\n",
						time.Now().Format("15:04:05"),
						id,
						typ,
						output.Highlight(status),
						date,
						title,
					)
				}
			}
			first = false
			return nil
		})
	},
}

// pollLoop runs fn at the given interval until the context is cancelled.
func pollLoop(ctx context.Context, intervalSec int, fn func() error) error {
	if intervalSec <= 0 {
		intervalSec = 60
	}

	// Run immediately on start
	if err := fn(); err != nil {
		return err
	}

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	if output.IsInteractive() {
		fmt.Fprintf(os.Stderr, "Polling every %ds. Press Ctrl+C to stop.\n", intervalSec)
	}

	for {
		select {
		case <-ctx.Done():
			if output.IsInteractive() {
				fmt.Fprintln(os.Stderr, "\nStopped.")
			}
			return nil
		case <-ticker.C:
			if err := fn(); err != nil {
				return err
			}
		}
	}
}

func init() {
	feedShabCmd.Flags().String("rubric", "", "Filter by rubric codes (comma-separated, e.g. HR,SB)")
	feedShabCmd.Flags().Int("interval", 60, "Polling interval in seconds")

	feedEntscheidCmd.Flags().String("court", "", "Filter by court or canton (e.g. BGer, BVGer, ZH)")
	feedEntscheidCmd.Flags().Int("interval", 60, "Polling interval in seconds")

	feedParlCmd.Flags().String("type", "", "Filter by business type abbreviation (e.g. Mo, Ip, Po)")
	feedParlCmd.Flags().Int("interval", 60, "Polling interval in seconds")

	feedCmd.AddCommand(feedShabCmd)
	feedCmd.AddCommand(feedEntscheidCmd)
	feedCmd.AddCommand(feedParlCmd)
	rootCmd.AddCommand(feedCmd)
}
