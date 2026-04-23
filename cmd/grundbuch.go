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

var grundbuchCmd = &cobra.Command{
	Use:   "grundbuch",
	Short: "Swiss land register (Grundbuch / registre foncier)",
	Long: `Swiss land register and parcel lookups across all 26 cantons.

Parcel (Liegenschaft / immeuble) geometry and EGRID data is universally
available via the federal aggregated cadastre at api3.geo.admin.ch.

Owner lookup is cantonal and access varies widely: some cantons expose
owner names in a free viewer, others require SMS to a Swiss mobile phone,
federal eID (AGOV/SwissID), a professional Terravis/Intercapi convention,
or an in-person counter visit. Run 'chli grundbuch cantons' for the full
capability matrix or 'chli grundbuch canton XX' for one canton's detail.

The CLI never returns a certified extract (beglaubigter Grundbuchauszug);
those always require the paid, identity-verified canton flow.`,
}

var grundbuchParcelCmd = &cobra.Command{
	Use:   "parcel",
	Short: "Look up a parcel by EGRID, address, coordinate, or canton+number",
	Long: `Resolve a parcel and print its EGRID, canton, municipality, number, area,
and LV95/WGS84 coordinates. Works uniformly across all 26 cantons via the
federal aggregated cadastral layer.

Provide exactly one of:
  --egrid CH…                  federal parcel ID
  --address "Bundesplatz 3, Bern"
  --coord  E,N                 LV95 east,north (e.g. 2600123,1199987)
  --canton XX --number N       canton-local parcel number`,
	RunE: runGrundbuchParcel,
}

var grundbuchOwnerCmd = &cobra.Command{
	Use:   "owner",
	Short: "Look up parcel owner — returns canton-specific how-to",
	Long: `Report owner information for a parcel.

Phase 1 implementation: this command always prints the canton's capability
block (portal, auth, cost, legal basis). Live unauthenticated owner
lookups are not yet wired up — see 'chli grundbuch cantons' for which
cantons expose owner names publicly, and use their portal directly.

Provide exactly one of:
  --egrid CH…
  --canton XX --number N`,
	RunE: runGrundbuchOwner,
}

var grundbuchCantonCmd = &cobra.Command{
	Use:   "canton <CODE>",
	Short: "Show one canton's Grundbuch capability and how to obtain owner data",
	Args:  cobra.ExactArgs(1),
	RunE:  runGrundbuchCanton,
}

var grundbuchCantonsCmd = &cobra.Command{
	Use:   "cantons",
	Short: "Show the Grundbuch capability matrix across all 26 cantons",
	RunE:  runGrundbuchCantons,
}

func runGrundbuchParcel(cmd *cobra.Command, args []string) error {
	egrid, _ := cmd.Flags().GetString("egrid")
	address, _ := cmd.Flags().GetString("address")
	coord, _ := cmd.Flags().GetString("coord")
	cantonFlag, _ := cmd.Flags().GetString("canton")
	number, _ := cmd.Flags().GetString("number")

	inputs := 0
	for _, v := range []string{egrid, address, coord} {
		if v != "" {
			inputs++
		}
	}
	if cantonFlag != "" && number != "" {
		inputs++
	}
	if inputs != 1 {
		return fmt.Errorf("provide exactly one of --egrid, --address, --coord, or --canton+--number")
	}

	client, err := api.NewClient()
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
	client.NoCache = noCache
	client.Refresh = refresh

	var parcel *api.Parcel

	switch {
	case egrid != "":
		parcel, err = client.GrundbuchFindByEGRID(egrid)
	case address != "":
		hits, searchErr := client.GrundbuchSearchAddress(address)
		if searchErr != nil {
			err = searchErr
			break
		}
		if len(hits) == 0 {
			return fmt.Errorf("no match for address %q", address)
		}
		parcel, err = client.GrundbuchIdentifyByHit(hits[0])
	case coord != "":
		e, n, coordErr := parseLV95Coord(coord)
		if coordErr != nil {
			return coordErr
		}
		parcel, err = client.GrundbuchIdentifyByCoord(e, n)
	case cantonFlag != "" && number != "":
		return fmt.Errorf("--canton+--number lookup is not yet implemented in Phase 1 — use --address or --egrid")
	}

	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
	if parcel == nil {
		return fmt.Errorf("no parcel resolved")
	}

	headers := []string{"Field", "Value"}
	rows := [][]string{
		{"EGRID", parcel.EGRID},
		{"Canton", parcel.Canton},
		{"Municipality", formatMuni(parcel)},
		{"Parcel #", parcel.Number},
		{"Area", formatArea(parcel.AreaM2)},
		{"LV95", formatLV95(parcel.LV95E, parcel.LV95N)},
		{"Portal", parcel.Portal},
	}
	output.Render(headers, rows, parcel)
	return nil
}

func runGrundbuchOwner(cmd *cobra.Command, args []string) error {
	egrid, _ := cmd.Flags().GetString("egrid")
	cantonFlag, _ := cmd.Flags().GetString("canton")
	number, _ := cmd.Flags().GetString("number")

	if (egrid == "" && cantonFlag == "") || (egrid != "" && cantonFlag != "") {
		return fmt.Errorf("provide either --egrid or --canton+--number")
	}

	client, err := api.NewClient()
	if err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
	client.NoCache = noCache
	client.Refresh = refresh

	var parcel *api.Parcel
	if egrid != "" {
		parcel, err = client.GrundbuchFindByEGRID(egrid)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
	} else {
		if number == "" {
			return fmt.Errorf("--canton also requires --number")
		}
		code, err := api.CantonCode(cantonFlag)
		if err != nil {
			return err
		}
		parcel = &api.Parcel{Canton: code, Number: number}
	}

	cap, ok := api.Cantons[parcel.Canton]
	if !ok {
		return fmt.Errorf("could not determine canton for this parcel")
	}

	// Phase 1: no live adapters. Always print capability block.
	renderOwnerHowTo(parcel, cap)
	return nil
}

func runGrundbuchCanton(cmd *cobra.Command, args []string) error {
	code, err := api.CantonCode(args[0])
	if err != nil {
		return err
	}
	cap := api.Cantons[code]
	renderCantonDetail(cap)
	return nil
}

func runGrundbuchCantons(cmd *cobra.Command, args []string) error {
	headers := []string{"Code", "Name", "Tier", "Parcel", "Owner-Public", "Auth", "Cost"}
	list := api.OrderedCantons()
	rows := make([][]string, 0, len(list))
	for _, c := range list {
		rows = append(rows, []string{
			c.Code,
			c.LocalizedName(output.Lang),
			c.Tier.String(),
			"yes",
			ownerPublicSummary(c),
			string(c.AuthModel),
			formatCost(c.Cost),
		})
	}
	output.Render(headers, rows, list)
	return nil
}

func renderOwnerHowTo(parcel *api.Parcel, cap api.CantonCapability) {
	// Structured "how-to" block. In JSON mode, emit capability+parcel verbatim.
	payload := map[string]any{
		"parcel":     parcel,
		"capability": cap,
		"note":       "Phase 1: no live owner adapters. Use the listed portal to obtain owner data.",
	}

	if !output.IsInteractive() {
		output.JSON(payload)
		return
	}

	fmt.Printf("\nCanton %s (%s) — %s\n", cap.Code, cap.LocalizedName(output.Lang), cap.Tier)
	if parcel.EGRID != "" {
		fmt.Printf("Parcel: EGRID %s", parcel.EGRID)
		if parcel.Municipality != "" {
			fmt.Printf(" — %s", parcel.Municipality)
		}
		if parcel.Number != "" {
			fmt.Printf(" #%s", parcel.Number)
		}
		fmt.Println()
	} else if parcel.Number != "" {
		fmt.Printf("Parcel: %s #%s (EGRID unresolved)\n", cap.Code, parcel.Number)
	}

	switch cap.Tier {
	case api.TierT1, api.TierT2:
		fmt.Printf("\n%s exposes owner data through a public viewer:\n", cap.Code)
		if cap.OwnerPublic != nil {
			fmt.Printf("  %s\n", cap.OwnerPublic.URL)
			if cap.OwnerPublic.Notes != "" {
				fmt.Printf("  %s\n", cap.OwnerPublic.Notes)
			}
		}
	default:
		fmt.Printf("\n%s does not expose owner data via a public API.\n", cap.Code)
	}

	fmt.Println("\nTo obtain an official extract:")
	for _, p := range cap.OwnerOrder {
		fmt.Printf("  %-36s %s\n", p.Name, p.URL)
	}

	fmt.Printf("\n  Auth          %s\n", cap.AuthModel)
	fmt.Printf("  Cost          %s\n", formatCost(cap.Cost))
	if cap.Cost.Notes != "" {
		fmt.Printf("                %s\n", cap.Cost.Notes)
	}
	if cap.LegalNotes != "" {
		fmt.Printf("  Legal basis   %s\n", cap.LegalNotes)
	}
	fmt.Printf("  Grundbuchamt  %s\n", cap.GrundbuchamtURL)
	fmt.Printf("\nVerified: %s\n", cap.VerifiedAt)
	for _, cav := range cap.Caveats {
		fmt.Printf("  ⚠ %s\n", cav)
	}
}

func renderCantonDetail(cap api.CantonCapability) {
	if !output.IsInteractive() {
		output.JSON(cap)
		return
	}

	fmt.Printf("\n%s — %s (%s)\n", cap.Code, cap.LocalizedName(output.Lang), cap.Tier)
	fmt.Println(strings.Repeat("-", 50))

	fmt.Printf("Parcel portal    %s (%s)\n                 %s\n", cap.ParcelPortal.Name, cap.ParcelPortal.Type, cap.ParcelPortal.URL)

	if cap.OwnerPublic != nil {
		fmt.Printf("Owner (public)   %s\n                 %s\n", cap.OwnerPublic.Type, cap.OwnerPublic.URL)
		if cap.OwnerPublic.Notes != "" {
			fmt.Printf("                 %s\n", cap.OwnerPublic.Notes)
		}
	} else {
		fmt.Println("Owner (public)   — (no unauthenticated endpoint)")
	}

	fmt.Println("\nOfficial extract channels:")
	for _, p := range cap.OwnerOrder {
		fmt.Printf("  %-36s %s\n", p.Name, p.URL)
	}

	fmt.Printf("\nAuth model       %s\n", cap.AuthModel)
	fmt.Printf("Cost             %s\n", formatCost(cap.Cost))
	if cap.Cost.Notes != "" {
		fmt.Printf("                 %s\n", cap.Cost.Notes)
	}
	if cap.LegalNotes != "" {
		fmt.Printf("Legal basis      %s\n", cap.LegalNotes)
	}
	fmt.Printf("Grundbuchamt     %s\n", cap.GrundbuchamtURL)
	fmt.Printf("Verified at      %s\n", cap.VerifiedAt)

	if len(cap.Caveats) > 0 {
		fmt.Println("\nCaveats:")
		for _, cav := range cap.Caveats {
			fmt.Printf("  ⚠ %s\n", cav)
		}
	}
}

func ownerPublicSummary(c api.CantonCapability) string {
	if c.OwnerPublic == nil {
		return "—"
	}
	switch c.AuthModel {
	case api.AuthNone:
		return "yes (viewer)"
	case api.AuthSMSPhone:
		return "SMS-gated"
	case api.AuthAGOV:
		return "AGOV login"
	case api.AuthSwissID:
		return "SwissID"
	}
	return "yes"
}

func formatCost(c api.CostSpec) string {
	switch {
	case c.FixedCHF != nil:
		return fmt.Sprintf("CHF %d", *c.FixedCHF)
	case c.MinCHF != nil && c.MaxCHF != nil:
		return fmt.Sprintf("CHF %d–%d", *c.MinCHF, *c.MaxCHF)
	case c.MinCHF != nil:
		return fmt.Sprintf("CHF %d+", *c.MinCHF)
	case c.Unpriced:
		return "verify at portal"
	}
	return "unpriced"
}

func formatMuni(p *api.Parcel) string {
	if p.Municipality == "" {
		return ""
	}
	if p.BFS > 0 {
		return fmt.Sprintf("%s (BFS %d)", p.Municipality, p.BFS)
	}
	return p.Municipality
}

func formatArea(m2 int) string {
	if m2 <= 0 {
		return ""
	}
	return fmt.Sprintf("%d m²", m2)
}

func formatLV95(e, n float64) string {
	if e == 0 && n == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f, %.0f", e, n)
}

// parseLV95Coord parses "E,N" in LV95 (Swiss CH1903+/LV95). Rejects values
// that look like WGS84 (lat/lon) with a clear error pointing to --address.
func parseLV95Coord(s string) (float64, float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("--coord must be 'E,N' in LV95 (e.g. 2600123,1199987)")
	}
	e, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid LV95 east: %w", err)
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid LV95 north: %w", err)
	}
	if e < 100 && n < 100 {
		return 0, 0, fmt.Errorf("coordinate looks like WGS84 lat/lon; use --address instead, or pass LV95 east,north (e.g. 2600123,1199987)")
	}
	if e < 2_450_000 || e > 2_850_000 || n < 1_070_000 || n > 1_300_000 {
		return 0, 0, fmt.Errorf("coordinate (%.0f, %.0f) is outside Swiss LV95 bounds", e, n)
	}
	return e, n, nil
}

func init() {
	grundbuchParcelCmd.Flags().String("egrid", "", "Federal parcel ID (EGRID)")
	grundbuchParcelCmd.Flags().String("address", "", "Postal address (e.g. 'Bundesplatz 3, Bern')")
	grundbuchParcelCmd.Flags().String("coord", "", "LV95 coordinate 'E,N' (e.g. 2600123,1199987)")
	grundbuchParcelCmd.Flags().String("canton", "", "Canton code (e.g. ZH) — use with --number")
	grundbuchParcelCmd.Flags().String("number", "", "Canton-local parcel number — use with --canton")

	grundbuchOwnerCmd.Flags().String("egrid", "", "Federal parcel ID (EGRID)")
	grundbuchOwnerCmd.Flags().String("canton", "", "Canton code (e.g. ZH)")
	grundbuchOwnerCmd.Flags().String("number", "", "Canton-local parcel number")

	grundbuchCmd.AddCommand(grundbuchParcelCmd)
	grundbuchCmd.AddCommand(grundbuchOwnerCmd)
	grundbuchCmd.AddCommand(grundbuchCantonCmd)
	grundbuchCmd.AddCommand(grundbuchCantonsCmd)
	rootCmd.AddCommand(grundbuchCmd)
}
