package main

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
)

var (
	fetchProviders string
	fetchDryRun    bool
)

func newFetchPricesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch-prices",
		Short: "Fetch model token pricing from Portkey-AI/models and store in the DB",
		Long: `Fetches per-model token pricing JSON from https://configs.portkey.ai/pricing
for the supported providers (openai, deepseek, anthropic, gemini, glm, kimi)
and upserts them into the model_pricings table.

The pricing data is used to compute cost in request logs. Re-run any time
to refresh — rows are updated in place.

Provider name mapping (internal -> Portkey):
  openai    -> openai
  deepseek  -> deepseek
  anthropic -> anthropic
  gemini    -> google
  glm       -> zhipu
  kimi      -> moonshot
`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return persistentLoad()
		},
		RunE: runFetchPrices,
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&fetchProviders, "provider", "", "comma-separated list of providers (default: all)")
	cmd.Flags().BoolVar(&fetchDryRun, "dry-run", false, "parse and print rows without writing to DB")
	return cmd
}

func runFetchPrices(cmd *cobra.Command, args []string) error {
	// Ensure SQLite WAL/journal is flushed before exit.
	defer database.Close()

	providers := commaList(fetchProviders)
	if len(providers) == 0 {
		providers = service.AllPortkeyProviders()
	}

	// Run migration to ensure the table exists (also done by `serve`, but
	// fetch-prices may be run on a fresh DB without starting the server).
	if !fetchDryRun {
		if err := database.Migrate(&models.ModelPricing{}); err != nil {
			return fmt.Errorf("failed to migrate model_pricings: %w", err)
		}
	}

	svc := service.NewPricingService(database.GetDB())

	start := time.Now()
	count, err := svc.FetchAndStore(providers, fetchDryRun)
	elapsed := time.Since(start)

	if fetchDryRun {
		log.Printf("[dry-run] parsed %d rows from %d providers in %s (not written)", count, len(providers), elapsed)
		return nil
	}

	if err != nil {
		log.Printf("Completed with errors: synced %d rows from %d providers in %s — %v", count, len(providers), elapsed, err)
		return err
	}
	log.Printf("Synced %d rows from %d providers in %s", count, len(providers), elapsed)
	return nil
}
