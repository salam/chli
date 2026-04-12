package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var shabCmd = &cobra.Command{
	Use:   "shab",
	Short: "Swiss Official Gazette (SHAB/FOSC) publications",
	Long:  "Search and view publications from the Swiss Official Gazette of Commerce (Schweizerisches Handelsamtsblatt).",
}

var shabSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search SHAB publications",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		rubricFlag, _ := cmd.Flags().GetString("rubric")
		page, _ := cmd.Flags().GetInt("page")
		size, _ := cmd.Flags().GetInt("size")

		var rubrics []string
		if rubricFlag != "" {
			rubrics = strings.Split(rubricFlag, ",")
		}

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		result, err := client.SHABSearch(query, rubrics, page, size)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(result)
			return nil
		}

		fmt.Printf("Found %d publications (page %d, showing %d)\n\n",
			result.Total, result.PageRequest.Page, len(result.Content))

		if len(result.Content) == 0 {
			fmt.Println("No results.")
			return nil
		}

		headers := []string{"Number", "Rubric", "Title", "Date", "Canton", "URL"}
		rows := make([][]string, 0, len(result.Content))
		for _, pub := range result.Content {
			m := pub.Meta
			title := output.Truncate(m.Title.Pick(output.Lang), 60)
			date := ""
			if len(m.PublicationDate) >= 10 {
				date = m.PublicationDate[:10]
			}
			cantons := strings.Join(m.Cantons, ",")
			url := api.SHABPublicationURL(m.ID)
			rows = append(rows, []string{
				m.PublicationNumber,
				m.Rubric,
				title,
				date,
				cantons,
				output.Hyperlink(url, m.PublicationNumber),
			})
		}
		output.Table(headers, rows)
		fmt.Println("\nUse: chli shab publication <number> for details")
		return nil
	},
}

var shabPublicationCmd = &cobra.Command{
	Use:   "publication <number>",
	Short: "Fetch a single SHAB publication by number or UUID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		pub, raw, err := client.SHABPublicationParsed(id)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.ForceJSON || !output.IsInteractive() {
			tree, mapErr := api.XMLToMap(raw)
			if mapErr != nil {
				output.Error(mapErr.Error())
				os.Exit(1)
			}
			output.JSON(tree)
			return nil
		}

		// Fall back to raw XML if parsing failed
		if pub == nil {
			fmt.Println(string(raw))
			return nil
		}

		// Interactive display
		m := pub.Meta
		url := api.SHABPublicationURL(m.ID)

		if m.PublicationNumber != "" {
			fmt.Printf("Publication:  %s\n", m.PublicationNumber)
		}
		if m.PublicationDate != "" {
			fmt.Printf("Date:         %s\n", m.PublicationDate)
		}
		if m.Rubric != "" {
			label := m.Rubric
			if m.SubRubric != "" {
				label += " / " + m.SubRubric
			}
			fmt.Printf("Rubric:       %s\n", label)
		}
		if m.Cantons != "" {
			fmt.Printf("Cantons:      %s\n", m.Cantons)
		}
		if title := m.Title.Pick(output.Lang); title != "" {
			fmt.Printf("Title:        %s\n", title)
		}
		if m.RegistrationOffice != nil && m.RegistrationOffice.DisplayName != "" {
			fmt.Printf("Office:       %s\n", m.RegistrationOffice.DisplayName)
		}
		if url != "" {
			fmt.Printf("URL:          %s\n", output.Hyperlink(url, url))
		}
		fmt.Println()

		if cn := pub.Content.CommonsNew; cn != nil && cn.Company != nil {
			co := cn.Company
			fmt.Printf("Company:      %s", co.Name)
			if co.UID != "" {
				fmt.Printf("  (%s)", co.UID)
			}
			fmt.Println()
			if co.Seat != "" || co.LegalForm != "" {
				line := co.Seat
				if co.LegalForm != "" {
					if line != "" {
						line += "  "
					}
					line += "legal form " + co.LegalForm
				}
				fmt.Printf("Seat:         %s\n", line)
			}
			if a := co.Address; a != nil {
				parts := []string{}
				if a.Street != "" {
					s := a.Street
					if a.HouseNumber != "" {
						s += " " + a.HouseNumber
					}
					parts = append(parts, s)
				}
				if a.SwissZipCode != "" || a.Town != "" {
					parts = append(parts, strings.TrimSpace(a.SwissZipCode+" "+a.Town))
				}
				if len(parts) > 0 {
					fmt.Printf("Address:      %s\n", strings.Join(parts, ", "))
				}
			}
			if cn.Revision != nil && cn.Revision.RevisionCompany != nil {
				rc := cn.Revision.RevisionCompany
				line := rc.Name
				if rc.UID != "" {
					line += "  (" + rc.UID + ")"
				}
				fmt.Printf("Auditor:      %s\n", line)
			}
			fmt.Println()
		}

		if tx := pub.Content.Transaction; tx != nil && tx.Update != nil {
			if labels := tx.Update.Changements.ChangedLabels(); len(labels) > 0 {
				fmt.Printf("Changes:      %s\n\n", strings.Join(labels, ", "))
			}
		}

		text := pub.Content.PublicationText.PickText(output.Lang)
		if text == "" && pub.Content.Message != "" {
			text = pub.Content.Message
		}
		if text != "" {
			fmt.Println(strings.TrimSpace(text))
		} else {
			fmt.Println(string(raw))
		}
		return nil
	},
}

var shabRubricsCmd = &cobra.Command{
	Use:   "rubrics",
	Short: "List all SHAB rubric codes",
	RunE: func(cmd *cobra.Command, args []string) error {
		type rubricEntry struct {
			Code string
			Name string
		}
		rubrics := []rubricEntry{
			{"HR", "Handelsregister"},
			{"SB", "Schuldbetreibung/Konkurs"},
			{"KK", "Konkurse"},
			{"AB", "Amtliche Bekanntmachungen"},
			{"AW", "Amtliche Warnungen"},
			{"BB", "Bundesblatt"},
			{"EK", "Eidg. Kommissionen"},
			{"ES", "Eidg. Steuerverwaltung"},
			{"FM", "Finanzmarktaufsicht"},
			{"LS", "Liegenschaftsschaetzungen"},
			{"NA", "Nachlassverfahren"},
			{"SR", "Sozialversicherungsrecht"},
			{"UP", "Umweltschutz"},
			{"UV", "Urheberrecht"},
			{"AZ", "Other"},
			{"BH", "Bundesamt fuer Gesundheit"},
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(rubrics)
			return nil
		}

		headers := []string{"Code", "Description"}
		rows := make([][]string, 0, len(rubrics))
		for _, r := range rubrics {
			rows = append(rows, []string{r.Code, r.Name})
		}
		output.Table(headers, rows)
		return nil
	},
}

func init() {
	shabSearchCmd.Flags().String("rubric", "", "Filter by rubric codes (comma-separated, e.g. HR,SB)")
	shabSearchCmd.Flags().Int("page", 0, "Page number (0-based)")
	shabSearchCmd.Flags().Int("size", 10, "Results per page")

	shabCmd.AddCommand(shabSearchCmd)
	shabCmd.AddCommand(shabPublicationCmd)
	shabCmd.AddCommand(shabRubricsCmd)
	rootCmd.AddCommand(shabCmd)
}
