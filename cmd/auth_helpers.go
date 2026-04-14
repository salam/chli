package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/matthiasak/chli/config"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

// authBinding describes how a user-facing `<source> login` subcommand maps
// to an underlying credentials store entry and environment variables. Two
// bindings may share the same Service value — e.g. `uid login` and
// `zefix login` both write to the "zefix" key because `uid lookup` goes
// through Zefix.
type authBinding struct {
	Service  string // key in config.Credentials.Services
	EnvUser  string
	EnvPass  string
	HelpLong string
}

// newLoginCmd builds a `login` subcommand that prompts for Basic auth
// credentials and persists them under b.Service.
func newLoginCmd(b authBinding) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store HTTP Basic credentials",
		Long:  b.HelpLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			user, _ := cmd.Flags().GetString("user")
			password, _ := cmd.Flags().GetString("password")

			if user == "" {
				fmt.Fprint(os.Stderr, "User: ")
				reader := bufio.NewReader(os.Stdin)
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading user: %w", err)
				}
				user = strings.TrimSpace(line)
			}
			if user == "" {
				return fmt.Errorf("user must not be empty")
			}
			if password == "" {
				p, err := readSecret("Password: ")
				if err != nil {
					return err
				}
				password = p
			}
			if password == "" {
				return fmt.Errorf("password must not be empty")
			}

			creds, err := config.LoadCredentials()
			if err != nil {
				return err
			}
			creds.Set(b.Service, config.ServiceCreds{User: user, Password: password})
			if err := creds.Save(); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}
			if output.IsInteractive() {
				fmt.Printf("Saved credentials for %s to %s\n", b.Service, config.CredentialsPath())
			} else {
				output.JSON(map[string]string{"service": b.Service, "stored": "ok", "path": config.CredentialsPath()})
			}
			return nil
		},
	}
	cmd.Flags().String("user", "", "Username (prompted if omitted)")
	cmd.Flags().String("password", "", "Password (prompted if omitted — prefer the prompt)")
	return cmd
}

// newLogoutCmd builds a `logout` subcommand that removes credentials for
// b.Service from the store.
func newLogoutCmd(b authBinding) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := config.LoadCredentials()
			if err != nil {
				return err
			}
			if _, ok := creds.Services[b.Service]; !ok {
				return fmt.Errorf("no stored credentials for %s", b.Service)
			}
			creds.Delete(b.Service)
			if err := creds.Save(); err != nil {
				return err
			}
			if output.IsInteractive() {
				fmt.Printf("Removed credentials for %s\n", b.Service)
			} else {
				output.JSON(map[string]string{"service": b.Service, "removed": "ok"})
			}
			return nil
		},
	}
}

// newStatusCmd builds a `status` subcommand that reports whether
// credentials are available — either via env vars or the keystore.
func newStatusCmd(b authBinding) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether credentials are configured",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := config.LoadCredentials()
			if err != nil {
				return err
			}
			c := creds.Get(b.Service)
			envUser := os.Getenv(b.EnvUser)
			envSet := envUser != "" && os.Getenv(b.EnvPass) != ""
			hasStored := c.User != ""

			source := "-"
			user := ""
			switch {
			case envSet:
				source = "env"
				user = envUser
			case hasStored:
				source = "keystore"
				user = c.User
			}
			stored := "no"
			if hasStored {
				stored = "yes"
			}
			envLabelVal := b.EnvUser
			if envSet {
				envLabelVal = b.EnvUser + "=set"
			}
			headers := []string{"Service", "Stored", "Env Var", "User"}
			rows := [][]string{{b.Service, stored, envLabelVal, user}}
			data := map[string]any{
				"service":         b.Service,
				"has_credentials": hasStored || envSet,
				"source":          source,
				"user":            user,
				"path":            config.CredentialsPath(),
			}
			output.Render(headers, rows, data)
			return nil
		},
	}
}
