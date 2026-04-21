package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var vehiclesCmd = &cobra.Command{
	Use:   "vehicles",
	Short: "Swiss vehicle statistics (BFS MOFIS via opendata.swiss)",
	Long: `Aggregate vehicle stock and new-registration statistics from the
Bundesamt für Statistik (MOFIS) dataset on opendata.swiss.

Filter by canton, fuel, vehicle type, make, and period; pivot by any
dimension; keep the top N rows. All data is fetched as CSV and processed
locally with a 24-hour cache.`,
}

// --- shared flag parsing ----------------------------------------------------

// parseCantonsFlag splits the comma-separated --canton value, uppercases
// everything, and validates against the set of 26.
func parseCantonsFlag(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		norm, err := api.NormalizeCanton(p)
		if err != nil {
			return nil, err
		}
		out = append(out, norm)
	}
	return out, nil
}

func validateGroupBy(v string) error {
	if v == "" {
		return nil
	}
	switch strings.ToLower(v) {
	case "canton", "fuel", "make", "type":
		return nil
	}
	return fmt.Errorf("invalid --group-by %q (valid: canton, fuel, make, type)", v)
}

// --- stock ------------------------------------------------------------------

var vehiclesStockCmd = &cobra.Command{
	Use:   "stock",
	Short: "Vehicle stock snapshot (quarterly, by canton/fuel/type/make)",
	Long: `Query the BFS quarterly vehicle stock dataset. Filters can be combined
freely; --as-of defaults to the latest available quarter.

Examples:
  chli vehicles stock --canton ZH --fuel electric
  chli vehicles stock --canton ZH,BE,GE --make Tesla --as-of 2026-Q1
  chli vehicles stock --fuel electric --group-by canton --top 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cantons, err := parseCantonsFlag(mustString(cmd, "canton"))
		if err != nil {
			return err
		}
		fuel, err := api.NormalizeFuel(mustString(cmd, "fuel"))
		if err != nil {
			return err
		}
		vtype, err := api.NormalizeVehicleType(mustString(cmd, "type"))
		if err != nil {
			return err
		}
		groupBy := strings.ToLower(strings.TrimSpace(mustString(cmd, "group-by")))
		if err := validateGroupBy(groupBy); err != nil {
			return err
		}
		topN, _ := cmd.Flags().GetInt("top")

		q := api.StockQuery{
			Cantons: cantons,
			Fuel:    fuel,
			Type:    vtype,
			Make:    mustString(cmd, "make"),
			AsOf:    mustString(cmd, "as-of"),
			GroupBy: groupBy,
			TopN:    topN,
		}

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		rows, err := client.FetchVehicleStock(context.Background(), q)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		renderStock(rows, q)
		return nil
	},
}

func renderStock(rows []api.StockRow, q api.StockQuery) {
	period := q.AsOf
	if period == "" && len(rows) > 0 {
		period = rows[0].Period
	}
	if q.GroupBy != "" {
		grouped := api.GroupAndTopN(rows, q.GroupBy, q.TopN)
		renderGrouped(period, q.GroupBy, grouped, map[string]any{
			"query":   q,
			"rows":    grouped,
			"period":  period,
			"kind":    "stock",
			"source":  "BFS MOFIS (opendata.swiss)",
		})
		return
	}

	// No grouping — show raw rows, truncated to TopN by count desc.
	sorted := append([]api.StockRow(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })
	if q.TopN > 0 && len(sorted) > q.TopN {
		sorted = sorted[:q.TopN]
	}

	headers := []string{"Period", "Canton", "Type", "Fuel", "Make", "Count"}
	tbl := make([][]string, 0, len(sorted)+1)
	total := 0
	for _, r := range sorted {
		tbl = append(tbl, []string{
			r.Period, r.Canton, r.Type, r.Fuel, r.Make, strconv.Itoa(r.Count),
		})
		total += r.Count
	}
	tbl = append(tbl, []string{"", "", "", "", "TOTAL", strconv.Itoa(total)})

	output.Render(headers, tbl, map[string]any{
		"query":  q,
		"rows":   sorted,
		"total":  total,
		"period": period,
		"kind":   "stock",
		"source": "BFS MOFIS (opendata.swiss)",
	})
}

// --- registrations ----------------------------------------------------------

var vehiclesRegistrationsCmd = &cobra.Command{
	Use:     "registrations",
	Aliases: []string{"reg"},
	Short:   "New vehicle registrations (monthly)",
	Long: `Query the BFS monthly new-registrations dataset. --from/--to accept
YYYY-MM; --from alone rolls forward to the latest available month.

Examples:
  chli vehicles registrations --canton ZH --make Tesla --from 2026-01 --to 2026-03
  chli vehicles registrations --fuel electric --from 2025-01
  chli vehicles registrations --group-by fuel --top 10`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cantons, err := parseCantonsFlag(mustString(cmd, "canton"))
		if err != nil {
			return err
		}
		fuel, err := api.NormalizeFuel(mustString(cmd, "fuel"))
		if err != nil {
			return err
		}
		vtype, err := api.NormalizeVehicleType(mustString(cmd, "type"))
		if err != nil {
			return err
		}
		groupBy := strings.ToLower(strings.TrimSpace(mustString(cmd, "group-by")))
		if err := validateGroupBy(groupBy); err != nil {
			return err
		}
		topN, _ := cmd.Flags().GetInt("top")

		q := api.RegistrationsQuery{
			Cantons: cantons,
			Fuel:    fuel,
			Type:    vtype,
			Make:    mustString(cmd, "make"),
			From:    mustString(cmd, "from"),
			To:      mustString(cmd, "to"),
			GroupBy: groupBy,
			TopN:    topN,
		}

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		rows, err := client.FetchRegistrations(context.Background(), q)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		renderRegistrations(rows, q)
		return nil
	},
}

func renderRegistrations(rows []api.RegistrationRow, q api.RegistrationsQuery) {
	period := registrationsPeriodLabel(rows, q)

	if q.GroupBy != "" {
		grouped := api.GroupAndTopN(rows, q.GroupBy, q.TopN)
		renderGrouped(period, q.GroupBy, grouped, map[string]any{
			"query":  q,
			"rows":   grouped,
			"period": period,
			"kind":   "registrations",
			"source": "BFS MOFIS (opendata.swiss)",
		})
		return
	}

	sorted := append([]api.RegistrationRow(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })
	if q.TopN > 0 && len(sorted) > q.TopN {
		sorted = sorted[:q.TopN]
	}

	headers := []string{"Period", "Canton", "Type", "Fuel", "Make", "Count"}
	tbl := make([][]string, 0, len(sorted)+1)
	total := 0
	for _, r := range sorted {
		tbl = append(tbl, []string{
			r.Period, r.Canton, r.Type, r.Fuel, r.Make, strconv.Itoa(r.Count),
		})
		total += r.Count
	}
	tbl = append(tbl, []string{"", "", "", "", "TOTAL", strconv.Itoa(total)})

	output.Render(headers, tbl, map[string]any{
		"query":  q,
		"rows":   sorted,
		"total":  total,
		"period": period,
		"kind":   "registrations",
		"source": "BFS MOFIS (opendata.swiss)",
	})
}

// registrationsPeriodLabel returns a descriptive "YYYY-MM..YYYY-MM" label
// covering the actual window observed in the filtered rows. Empty when no
// rows were returned.
func registrationsPeriodLabel(rows []api.RegistrationRow, q api.RegistrationsQuery) string {
	if q.From != "" && q.To != "" {
		return q.From + ".." + q.To
	}
	if len(rows) == 0 {
		return ""
	}
	minP, maxP := rows[0].Period, rows[0].Period
	for _, r := range rows {
		if r.Period < minP {
			minP = r.Period
		}
		if r.Period > maxP {
			maxP = r.Period
		}
	}
	if minP == maxP {
		return minP
	}
	return minP + ".." + maxP
}

// renderGrouped is the shared grouped-output path for both subcommands.
func renderGrouped(period, groupBy string, grouped []api.GroupedRow, jsonPayload map[string]any) {
	headers := []string{"Period", strings.Title(groupBy), "Count"}
	tbl := make([][]string, 0, len(grouped)+1)
	total := 0
	for _, g := range grouped {
		tbl = append(tbl, []string{period, g.Key, strconv.Itoa(g.Count)})
		total += g.Count
	}
	tbl = append(tbl, []string{"", "TOTAL", strconv.Itoa(total)})
	jsonPayload["total"] = total
	output.Render(headers, tbl, jsonPayload)
}

// --- helpers ----------------------------------------------------------------

func mustString(cmd *cobra.Command, name string) string {
	s, _ := cmd.Flags().GetString(name)
	return s
}

func init() {
	// Shared filter flags (declared separately so each subcommand owns its set).
	for _, sub := range []*cobra.Command{vehiclesStockCmd, vehiclesRegistrationsCmd} {
		sub.Flags().String("canton", "", "Canton code(s), comma-separated (e.g. ZH,BE,GE)")
		sub.Flags().String("fuel", "", "Fuel: petrol|diesel|electric|hybrid|gas|hydrogen|other")
		sub.Flags().String("type", "", "Vehicle type: car|truck|motorcycle|bus|tractor|trailer")
		sub.Flags().String("make", "", "Make substring (case-insensitive)")
		sub.Flags().String("group-by", "", "Pivot dimension: canton|fuel|make|type")
		sub.Flags().Int("top", 20, "Keep top N rows after sorting/grouping (0 = no truncation)")
	}

	vehiclesStockCmd.Flags().String("as-of", "", "Snapshot period YYYY-QN (default: latest)")

	vehiclesRegistrationsCmd.Flags().String("from", "", "Start month YYYY-MM (inclusive)")
	vehiclesRegistrationsCmd.Flags().String("to", "", "End month YYYY-MM (inclusive; defaults to latest when --from set)")

	vehiclesCmd.AddCommand(vehiclesStockCmd)
	vehiclesCmd.AddCommand(vehiclesRegistrationsCmd)
	rootCmd.AddCommand(vehiclesCmd)
}
