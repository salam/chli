package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// --- grammar / introspection ---
//
// Walks the live cobra command tree and emits the CLI surface in either
// EBNF (human-readable spec) or JSON (machine-consumable — suitable for
// building completion parsers in web projects).
//
// Both outputs are derived from the same internal model, so they cannot
// drift from the real command definitions.

// argSpec — one positional argument parsed from Cobra's `Use` string.
type argSpec struct {
	Name     string   `json:"name"`
	Required bool     `json:"required"`
	Variadic bool     `json:"variadic"`
	Choices  []string `json:"choices,omitempty"`
}

type flagSpec struct {
	Name        string   `json:"name"`
	Short       string   `json:"short,omitempty"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	Choices     []string `json:"choices,omitempty"`
}

type cmdSpec struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"` // full path from root, e.g. "chli parl business"
	Usage       string     `json:"usage"`
	Short       string     `json:"short,omitempty"`
	Args        []argSpec  `json:"args,omitempty"`
	Flags       []flagSpec `json:"flags,omitempty"`
	Subcommands []cmdSpec  `json:"subcommands,omitempty"`
}

type rootSpec struct {
	Name        string     `json:"name"`
	Short       string     `json:"short,omitempty"`
	GlobalFlags []flagSpec `json:"globalFlags"`
	Commands    []cmdSpec  `json:"commands"`
}

// Cobra `Use` strings like: "bookmark add <type> <id> [label]"
// We split off the command name and treat the remainder as positional slots.
var argToken = regexp.MustCompile(`^(<[^>]+>|\[[^\]]+\])(\.\.\.)?$`)

func parseArgs(use string) []argSpec {
	fields := strings.Fields(use)
	if len(fields) <= 1 {
		return nil
	}
	var out []argSpec
	for _, tok := range fields[1:] {
		variadic := strings.HasSuffix(tok, "...")
		t := strings.TrimSuffix(tok, "...")
		if len(t) < 2 {
			continue
		}
		required := t[0] == '<'
		name := t[1 : len(t)-1]
		out = append(out, argSpec{Name: name, Required: required, Variadic: variadic})
	}
	return out
}

// choicesFromUsage pulls out "(a, b, c)" or "a|b|c" patterns from a flag's
// usage text — best-effort hint for completion parsers.
var choiceRE = regexp.MustCompile(`\(([a-zA-Z][\w,\s\-]*)\)`)

func flagChoices(usage string) []string {
	m := choiceRE.FindStringSubmatch(usage)
	if m == nil {
		return nil
	}
	parts := strings.Split(m[1], ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.Contains(p, " ") {
			// not a simple enum; bail
			return nil
		}
		out = append(out, p)
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

func collectFlags(fs *pflag.FlagSet) []flagSpec {
	var out []flagSpec
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		spec := flagSpec{
			Name:        f.Name,
			Short:       f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
			Choices:     flagChoices(f.Usage),
		}
		out = append(out, spec)
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func buildCmdSpec(c *cobra.Command, parentPath string) cmdSpec {
	name := strings.Fields(c.Use)
	head := c.Name()
	if len(name) > 0 {
		head = name[0]
	}
	path := head
	if parentPath != "" {
		path = parentPath + " " + head
	}
	spec := cmdSpec{
		Name:  head,
		Path:  path,
		Usage: c.Use,
		Short: c.Short,
		Args:  parseArgs(c.Use),
		Flags: collectFlags(c.LocalFlags()),
	}
	// Inject Cobra's own ValidArgs as a choice list on the first positional.
	if len(c.ValidArgs) > 0 && len(spec.Args) > 0 {
		spec.Args[0].Choices = append([]string(nil), c.ValidArgs...)
	}
	children := c.Commands()
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	for _, child := range children {
		if child.Hidden || child.Name() == "help" {
			continue
		}
		spec.Subcommands = append(spec.Subcommands, buildCmdSpec(child, path))
	}
	return spec
}

func buildRootSpec() rootSpec {
	root := rootSpec{
		Name:        rootCmd.Name(),
		Short:       rootCmd.Short,
		GlobalFlags: collectFlags(rootCmd.PersistentFlags()),
	}
	children := rootCmd.Commands()
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	for _, child := range children {
		if child.Hidden || child.Name() == "help" {
			continue
		}
		root.Commands = append(root.Commands, buildCmdSpec(child, rootCmd.Name()))
	}
	return root
}

// --- EBNF renderer ---

func ebnfIdent(path string) string {
	return strings.ReplaceAll(path, " ", "-") + "-cmd"
}

func ebnfArg(a argSpec) string {
	var tok string
	if len(a.Choices) > 0 {
		var qs []string
		for _, c := range a.Choices {
			qs = append(qs, fmt.Sprintf("%q", c))
		}
		tok = "( " + strings.Join(qs, " | ") + " )"
	} else {
		tok = strings.ToUpper(strings.ReplaceAll(a.Name, "-", "_"))
	}
	if a.Variadic {
		if a.Required {
			return tok + " , { " + tok + " }"
		}
		return "{ " + tok + " }"
	}
	if a.Required {
		return tok
	}
	return "[ " + tok + " ]"
}

func ebnfFlag(f flagSpec) string {
	var left string
	if f.Short != "" {
		left = fmt.Sprintf(`( "-%s" | "--%s" )`, f.Short, f.Name)
	} else {
		left = fmt.Sprintf(`"--%s"`, f.Name)
	}
	if f.Type == "bool" {
		return left
	}
	if len(f.Choices) > 0 {
		var qs []string
		for _, c := range f.Choices {
			qs = append(qs, fmt.Sprintf("%q", c))
		}
		return left + " , ( " + strings.Join(qs, " | ") + " )"
	}
	typeTok := strings.ToUpper(f.Type)
	return left + " , " + typeTok
}

func renderEBNF(r rootSpec, w *strings.Builder) {
	fmt.Fprintf(w, "(* %s — generated from the live cobra command tree *)\n\n", r.Name)

	fmt.Fprintf(w, "%s = %q , { global-flag } , [ command ] ;\n\n", r.Name, r.Name)

	// command alternation
	fmt.Fprintf(w, "command = ")
	names := make([]string, len(r.Commands))
	for i, c := range r.Commands {
		names[i] = ebnfIdent(c.Path)
	}
	fmt.Fprintf(w, "%s ;\n\n", strings.Join(names, "\n        | "))

	// global flags
	fmt.Fprintf(w, "global-flag = ")
	var parts []string
	for _, f := range r.GlobalFlags {
		parts = append(parts, ebnfFlag(f))
	}
	fmt.Fprintf(w, "%s ;\n\n", strings.Join(parts, "\n            | "))

	// commands, recursively
	var emit func(c cmdSpec)
	emit = func(c cmdSpec) {
		id := ebnfIdent(c.Path)
		fmt.Fprintf(w, "(* %s — %s *)\n", c.Path, c.Short)
		rhs := []string{fmt.Sprintf("%q", c.Name)}
		for _, a := range c.Args {
			rhs = append(rhs, ebnfArg(a))
		}
		if len(c.Subcommands) > 0 {
			var subs []string
			for _, s := range c.Subcommands {
				subs = append(subs, ebnfIdent(s.Path))
			}
			rhs = append(rhs, "( "+strings.Join(subs, " | ")+" )")
		}
		for _, f := range c.Flags {
			rhs = append(rhs, "{ "+ebnfFlag(f)+" }")
		}
		fmt.Fprintf(w, "%s = %s ;\n\n", id, strings.Join(rhs, " , "))
		for _, s := range c.Subcommands {
			emit(s)
		}
	}
	for _, c := range r.Commands {
		emit(c)
	}
}

// --- cobra wiring ---

var grammarFormat string

var grammarCmd = &cobra.Command{
	Use:   "grammar",
	Short: "Print the CLI grammar (introspected from the live command tree)",
	Long: `Print the full CLI surface — every subcommand, positional, and flag —
derived directly from the internal cobra command tree so it cannot drift
from the real definitions.

Formats:
  ebnf   EBNF-style grammar (default, human-readable)
  json   structured JSON, suitable for building completion parsers
         (includes command tree, args, flags with types/defaults/choices)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		spec := buildRootSpec()
		switch grammarFormat {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(spec)
		case "ebnf", "":
			var b strings.Builder
			renderEBNF(spec, &b)
			fmt.Print(b.String())
			return nil
		default:
			return fmt.Errorf("unknown format %q (want: ebnf, json)", grammarFormat)
		}
	},
}

func init() {
	grammarCmd.Flags().StringVar(&grammarFormat, "format", "ebnf", "Output format (ebnf, json)")
	rootCmd.AddCommand(grammarCmd)
}
