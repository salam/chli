package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var geoCmd = &cobra.Command{
	Use:   "geo",
	Short: "Swiss geoportal (geo.admin.ch)",
	Long:  "Search addresses and map features, list layers, and identify features at a coordinate via the geo.admin.ch API.",
}

var geoSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for addresses, places, or layers",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		typ, _ := cmd.Flags().GetString("type")
		limit, _ := cmd.Flags().GetInt("limit")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		result, err := client.GeoSearch(strings.Join(args, " "), typ, limit)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		headers := []string{"Layer", "Label", "Lat", "Lon"}
		rows := make([][]string, 0, len(result.Results))
		for _, r := range result.Results {
			rows = append(rows, []string{
				r.Attrs.Layer,
				output.Truncate(stripGeoHTML(r.Attrs.Label), 70),
				formatCoord(r.Attrs.Lat),
				formatCoord(r.Attrs.Lon),
			})
		}
		output.Render(headers, rows, result)
		return nil
	},
}

var geoLayersCmd = &cobra.Command{
	Use:   "layers",
	Short: "List available map layers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		result, err := client.GeoLayers()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		headers := []string{"Layer ID", "Type", "Attribution"}
		rows := make([][]string, 0, len(result.Layers))
		for _, l := range result.Layers {
			rows = append(rows, []string{l.LayerBodID, l.Type, output.Truncate(l.Attribution, 40)})
		}
		output.Render(headers, rows, result)
		return nil
	},
}

var geoIdentifyCmd = &cobra.Command{
	Use:   "identify <lon,lat>",
	Short: "Identify map features at a WGS84 coordinate",
	Long:  "Given a WGS84 coordinate (longitude,latitude), return features from map layers at that point. Use --layers to override.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := strings.Split(args[0], ",")
		if len(parts) != 2 {
			return fmt.Errorf("coordinate must be in form lon,lat (got %q)", args[0])
		}
		lon, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return fmt.Errorf("invalid longitude: %w", err)
		}
		lat, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return fmt.Errorf("invalid latitude: %w", err)
		}
		layers, _ := cmd.Flags().GetString("layers")

		client, err := api.NewClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		client.NoCache = noCache
		client.Refresh = refresh

		result, err := client.GeoIdentify(lon, lat, layers)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		headers := []string{"Layer", "Feature ID", "Name"}
		rows := make([][]string, 0, len(result.Results))
		for _, r := range result.Results {
			name := firstStringAttr(r.Attributes, "label", "name", "gemeinde", "strname1", "bezeichnung")
			rows = append(rows, []string{r.LayerBodID, string(r.FeatureID), output.Truncate(name, 60)})
		}
		output.Render(headers, rows, result)
		return nil
	},
}

func formatCoord(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 6, 64)
}

// stripGeoHTML does a minimal tag strip — geo.admin.ch labels embed <b>…</b>.
func stripGeoHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstStringAttr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func init() {
	geoSearchCmd.Flags().String("type", "locations", "Search type: locations, layers, featuresearch")
	geoSearchCmd.Flags().Int("limit", 15, "Max results")
	geoIdentifyCmd.Flags().String("layers", "", "Comma-separated layer IDs (default: a small curated set)")

	geoCmd.AddCommand(geoSearchCmd)
	geoCmd.AddCommand(geoLayersCmd)
	geoCmd.AddCommand(geoIdentifyCmd)
	rootCmd.AddCommand(geoCmd)
}
