package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
)

// global config path flag value — set on the root command, read by subcommands.
var configFile string

// loadedConfig is the config loaded in PersistentPreRunE; subcommands read it.
var loadedConfig *config.Config

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "llm-gateway",
		Short: "LLM Gateway — unified API gateway for multiple LLM providers",
		// Default behaviour: when invoked with no subcommand (or with an
		// unknown one that we treat as "serve"), start the server. This keeps
		// the old `./llm-gateway` invocation working.
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := persistentLoad(); err != nil {
				return err
			}
			return runServe(cmd, args)
		},
		// Disable cobra's automatic `help`/`completion` subcommands' error
		// printing when the user just runs the binary.
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "path to config.yaml (defaults to configs/config.yaml)")

	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newFetchPricesCmd())

	return rootCmd
}

// persistentLoad loads config + initializes the DB so subcommands can use them.
// Called from each subcommand's PreRunE (rather than rootCmd.PersistentPreRunE)
// so that `--help` and unknown args don't trigger DB connection.
func persistentLoad() error {
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Printf("Warning: Failed to load config: %v, using defaults", err)
		cfg = &config.Config{
			Server: config.ServerConfig{
				Host:         "0.0.0.0",
				Port:         8080,
				ReadTimeout:  60 * 1000 * 1000 * 1000, // 60s
				WriteTimeout: 60 * 1000 * 1000 * 1000,
			},
			Gateway: config.GatewayConfig{
				DefaultProvider:   "openai",
				GuardrailsEnabled: true,
			},
			Database: config.DatabaseConfig{
				Driver:   "sqlite",
				DSN:      "llm-gateway.db",
				LogLevel: "warn",
			},
		}
	}
	loadedConfig = cfg

	dbCfg := &database.Config{
		Driver:   cfg.Database.Driver,
		DSN:      cfg.Database.DSN,
		LogLevel: cfg.Database.LogLevel,
	}
	if err := database.Connect(dbCfg); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	return nil
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// cobra prints errors itself; exit non-zero.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// commaList splits a comma-separated CLI flag value into trimmed entries.
func commaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
