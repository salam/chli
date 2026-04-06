package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

// Bookmark represents a single bookmarked item.
type Bookmark struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Label       string `json:"label"`
	AddedAt     string `json:"added_at"`
	LastStatus  string `json:"last_status"`
	LastChecked string `json:"last_checked"`
}

func bookmarksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "chli", "bookmarks.json"), nil
}

func loadBookmarks() ([]Bookmark, error) {
	path, err := bookmarksPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Bookmark{}, nil
		}
		return nil, err
	}
	var bookmarks []Bookmark
	if err := json.Unmarshal(data, &bookmarks); err != nil {
		return nil, fmt.Errorf("parsing bookmarks: %w", err)
	}
	return bookmarks, nil
}

func saveBookmarks(bookmarks []Bookmark) error {
	path, err := bookmarksPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bookmarks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func findBookmark(bookmarks []Bookmark, typ, id string) int {
	for i, b := range bookmarks {
		if strings.EqualFold(b.Type, typ) && b.ID == id {
			return i
		}
	}
	return -1
}

var bookmarkCmd = &cobra.Command{
	Use:   "bookmark",
	Short: "Manage bookmarked items (parliament business, court decisions, etc.)",
	Long:  "Bookmark items to track them and check for updates later.",
}

var bookmarkAddCmd = &cobra.Command{
	Use:   "add <type> <id> [label]",
	Short: "Add a bookmark",
	Long: `Add a bookmark for a parliament business, court decision, or other item.

Types: business, entscheid, shab
Examples:
  chli bookmark add business 20.3456 "Motion Title"
  chli bookmark add entscheid abc-123 "BGer Decision"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		typ := args[0]
		id := args[1]
		label := ""
		if len(args) > 2 {
			label = strings.Join(args[2:], " ")
		}

		bookmarks, err := loadBookmarks()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if idx := findBookmark(bookmarks, typ, id); idx >= 0 {
			output.Error(fmt.Sprintf("Bookmark already exists: %s/%s", typ, id))
			os.Exit(1)
		}

		b := Bookmark{
			Type:    typ,
			ID:      id,
			Label:   label,
			AddedAt: time.Now().Format(time.RFC3339),
		}
		bookmarks = append(bookmarks, b)

		if err := saveBookmarks(bookmarks); err != nil {
			output.Error(fmt.Sprintf("saving bookmarks: %s", err))
			os.Exit(1)
		}

		if output.IsInteractive() {
			fmt.Printf("Bookmarked %s/%s", typ, id)
			if label != "" {
				fmt.Printf(" (%s)", label)
			}
			fmt.Println()
		} else {
			output.JSON(b)
		}
		return nil
	},
}

var bookmarkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all bookmarks",
	RunE: func(cmd *cobra.Command, args []string) error {
		bookmarks, err := loadBookmarks()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if len(bookmarks) == 0 {
			if output.IsInteractive() {
				fmt.Println("No bookmarks yet. Use 'chli bookmark add <type> <id>' to add one.")
			} else {
				output.JSON([]Bookmark{})
			}
			return nil
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(bookmarks)
			return nil
		}

		headers := []string{"Type", "ID", "Label", "Added", "Last Status", "Last Checked"}
		rows := make([][]string, 0, len(bookmarks))
		for _, b := range bookmarks {
			added := b.AddedAt
			if len(added) >= 10 {
				added = added[:10]
			}
			checked := b.LastChecked
			if len(checked) >= 10 {
				checked = checked[:10]
			}
			rows = append(rows, []string{
				b.Type,
				b.ID,
				output.Truncate(b.Label, 40),
				added,
				b.LastStatus,
				checked,
			})
		}
		output.Render(headers, rows, bookmarks)
		return nil
	},
}

var bookmarkRemoveCmd = &cobra.Command{
	Use:   "remove <type> <id>",
	Short: "Remove a bookmark",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		typ := args[0]
		id := args[1]

		bookmarks, err := loadBookmarks()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		idx := findBookmark(bookmarks, typ, id)
		if idx < 0 {
			output.Error(fmt.Sprintf("Bookmark not found: %s/%s", typ, id))
			os.Exit(1)
		}

		removed := bookmarks[idx]
		bookmarks = append(bookmarks[:idx], bookmarks[idx+1:]...)

		if err := saveBookmarks(bookmarks); err != nil {
			output.Error(fmt.Sprintf("saving bookmarks: %s", err))
			os.Exit(1)
		}

		if output.IsInteractive() {
			fmt.Printf("Removed bookmark %s/%s\n", typ, id)
		} else {
			output.JSON(removed)
		}
		return nil
	},
}

var bookmarkCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check bookmarked items for updates",
	Long:  "Re-fetch each bookmarked item and report any status changes since last check.",
	RunE: func(cmd *cobra.Command, args []string) error {
		bookmarks, err := loadBookmarks()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if len(bookmarks) == 0 {
			if output.IsInteractive() {
				fmt.Println("No bookmarks to check.")
			} else {
				output.JSON([]any{})
			}
			return nil
		}

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = true // Always fetch fresh data for checks

		type checkResult struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			Label     string `json:"label"`
			OldStatus string `json:"old_status"`
			NewStatus string `json:"new_status"`
			Changed   bool   `json:"changed"`
		}

		var results []checkResult
		now := time.Now().Format(time.RFC3339)

		for i := range bookmarks {
			b := &bookmarks[i]
			newStatus := ""

			switch b.Type {
			case "business":
				status, err := fetchBusinessStatus(client, b.ID)
				if err != nil {
					output.Verbosef("Error checking %s/%s: %s", b.Type, b.ID, err)
					continue
				}
				newStatus = status
			case "entscheid":
				status, err := fetchEntscheidStatus(client, b.ID)
				if err != nil {
					output.Verbosef("Error checking %s/%s: %s", b.Type, b.ID, err)
					continue
				}
				newStatus = status
			case "shab":
				// SHAB publications are static once published; just mark as checked
				newStatus = "published"
			default:
				output.Verbosef("Unknown bookmark type: %s", b.Type)
				continue
			}

			changed := b.LastStatus != "" && b.LastStatus != newStatus
			results = append(results, checkResult{
				Type:      b.Type,
				ID:        b.ID,
				Label:     b.Label,
				OldStatus: b.LastStatus,
				NewStatus: newStatus,
				Changed:   changed,
			})

			b.LastStatus = newStatus
			b.LastChecked = now
		}

		if err := saveBookmarks(bookmarks); err != nil {
			output.Error(fmt.Sprintf("saving bookmarks: %s", err))
			os.Exit(1)
		}

		if output.ForceJSON || !output.IsInteractive() {
			output.JSON(results)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No items could be checked.")
			return nil
		}

		headers := []string{"Type", "ID", "Label", "Old Status", "New Status", "Changed"}
		rows := make([][]string, 0, len(results))
		for _, r := range results {
			changedStr := ""
			if r.Changed {
				changedStr = output.Highlight("yes")
			}
			rows = append(rows, []string{
				r.Type,
				r.ID,
				output.Truncate(r.Label, 30),
				r.OldStatus,
				output.Highlight(r.NewStatus),
				changedStr,
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

// fetchBusinessStatus fetches the current status of a parliament business item.
func fetchBusinessStatus(client *api.Client, id string) (string, error) {
	var result struct {
		D struct {
			Results []struct {
				BusinessStatusText *string `json:"BusinessStatusText"`
			} `json:"results"`
		} `json:"d"`
	}
	filter := fmt.Sprintf("BusinessShortNumber eq '%s'", id)
	path := fmt.Sprintf("/odata.svc/Business?$filter=%s&$select=BusinessStatusText&$top=1", filter)
	if err := client.DoJSON("https://ws.parlament.ch", path, &result); err != nil {
		return "", err
	}
	if len(result.D.Results) == 0 {
		return "", fmt.Errorf("business %s not found", id)
	}
	if result.D.Results[0].BusinessStatusText != nil {
		return *result.D.Results[0].BusinessStatusText, nil
	}
	return "unknown", nil
}

// fetchEntscheidStatus fetches the current status of a court decision.
func fetchEntscheidStatus(client *api.Client, id string) (string, error) {
	var result struct {
		ID   string             `json:"_id"`
		Date string             `json:"date"`
		Title output.MultilingualText `json:"title"`
	}
	if err := client.DoJSON("https://entscheidsuche.ch", "/api/v1/docs/"+id, &result); err != nil {
		return "", err
	}
	if result.Date != "" {
		return "published:" + result.Date, nil
	}
	return "found", nil
}

func init() {
	bookmarkCmd.AddCommand(bookmarkAddCmd)
	bookmarkCmd.AddCommand(bookmarkListCmd)
	bookmarkCmd.AddCommand(bookmarkRemoveCmd)
	bookmarkCmd.AddCommand(bookmarkCheckCmd)
	rootCmd.AddCommand(bookmarkCmd)
}
