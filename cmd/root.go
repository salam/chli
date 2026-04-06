package cmd

import (
	"fmt"
	"os"

	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	noCache bool
	refresh bool
)

var rootCmd = &cobra.Command{
	Use:     "chli",
	Short:   "Unified CLI for Swiss government open data",
	Long:    "chli provides access to Swiss federal open data: parliament, federal law, court decisions, official gazette, and more.",
	Version: version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&output.ForceJSON, "json", false, "Force JSON output")
	rootCmd.PersistentFlags().BoolVar(&output.NoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVar(&output.Lang, "lang", "de", "Language (de, fr, it, en, rm)")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "Skip cache reads")
	rootCmd.PersistentFlags().BoolVar(&refresh, "refresh", false, "Force cache refresh")
}
