package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
	noCache   bool
	refresh   bool
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

// --- completion ---

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for chli.

To load completions:

  bash:
    source <(chli completion bash)
    # Or for permanent: chli completion bash > /etc/bash_completion.d/chli

  zsh:
    source <(chli completion zsh)
    # Or for permanent: chli completion zsh > "${fpath[1]}/_chli"

  fish:
    chli completion fish | source
    # Or for permanent: chli completion fish > ~/.config/fish/completions/chli.fish

  powershell:
    chli completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// --- version ---

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print detailed version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("chli %s\n", version)
		fmt.Printf("  commit:    %s\n", commit)
		fmt.Printf("  built:     %s\n", buildDate)
		fmt.Printf("  go:        %s\n", runtime.Version())
		fmt.Printf("  os/arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

// --- cache management ---

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local cache",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove all cached data",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(cfg.CacheDir)
		if err != nil {
			output.Error(fmt.Sprintf("reading cache dir: %s", err))
			os.Exit(1)
		}
		count := 0
		for _, e := range entries {
			path := cfg.CacheDir + "/" + e.Name()
			if err := os.Remove(path); err == nil {
				count++
			}
		}
		if output.IsInteractive() {
			fmt.Printf("Cleared %d cached entries from %s\n", count, cfg.CacheDir)
		} else {
			output.JSON(map[string]any{"cleared": count, "dir": cfg.CacheDir})
		}
		return nil
	},
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(cfg.CacheDir)
		if err != nil {
			output.Error(fmt.Sprintf("reading cache dir: %s", err))
			os.Exit(1)
		}
		var totalSize int64
		for _, e := range entries {
			info, err := e.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		if output.IsInteractive() {
			fmt.Printf("Cache directory: %s\n", cfg.CacheDir)
			fmt.Printf("Entries:         %d\n", len(entries))
			fmt.Printf("Total size:      %s\n", humanBytes(totalSize))
		} else {
			output.JSON(map[string]any{
				"dir":     cfg.CacheDir,
				"entries": len(entries),
				"bytes":   totalSize,
			})
		}
		return nil
	},
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func loadConfig() (*struct{ CacheDir string }, error) {
	// Use config package indirectly to avoid import cycle
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cacheDir := home + "/.cache/chli"
	return &struct{ CacheDir string }{CacheDir: cacheDir}, nil
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&output.ForceJSON, "json", false, "Force JSON output")
	rootCmd.PersistentFlags().BoolVar(&output.NoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVar(&output.Lang, "lang", "de", "Language (de, fr, it, en, rm)")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "Skip cache reads")
	rootCmd.PersistentFlags().BoolVar(&refresh, "refresh", false, "Force cache refresh")
	rootCmd.PersistentFlags().StringVarP(&output.OutputFormat, "output", "o", "", "Output format: json, csv, tsv")
	rootCmd.PersistentFlags().BoolVarP(&output.Quiet, "quiet", "q", false, "Suppress headers and section titles, output data rows only")
	rootCmd.PersistentFlags().StringVar(&output.Columns, "columns", "", "Comma-separated list of columns to display")
	rootCmd.PersistentFlags().BoolVarP(&output.Verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&output.Debug, "debug", false, "Debug output (implies verbose)")

	// Set version template to include build info
	rootCmd.SetVersionTemplate(fmt.Sprintf("chli %s (commit: %s, built: %s)\n", version, commit, buildDate))

	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheStatsCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(versionCmd)
}
