package main

import (
	"fmt"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/browser"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/history"
	"github.com/spf13/cobra"
)

func confirmCmd() *cobra.Command {
	var confirmURL string
	var brokerID string
	var pending bool
	var validateDomain bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "confirm",
		Short: "Open the confirmation links brokers email you",
		Long: `Opens the confirmation links data brokers send, which finishes an opt-out.

Each link is requested with a plain HTTP GET, redirects are followed, and the
page that comes back decides whether it worked.

Examples:
  # Confirm a specific URL
  delistd confirm --url "https://broker.com/confirm?token=abc123"

  # Confirm for a specific broker (using URL from pipeline)
  delistd confirm --broker spokeo

  # Confirm all pending confirmation links
  delistd confirm --pending

  # Preview without actually clicking (dry run)
  delistd confirm --pending --dry-run

Safeguards:
  - a link must point at a known broker domain, and so must every redirect
  - at most 10 redirects are followed
  - a page that reports a failure is never counted as a confirmation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfirm(confirmURL, brokerID, pending, validateDomain, dryRun)
		},
	}

	cmd.Flags().StringVar(&confirmURL, "url", "", "The confirmation link to open")
	cmd.Flags().StringVar(&brokerID, "broker", "", "Open the link a broker's reply carried (by broker ID)")
	cmd.Flags().BoolVar(&pending, "pending", false, "Open every confirmation link still waiting")
	cmd.Flags().BoolVar(&validateDomain, "validate-domain", true, "Validate URL domain against known brokers (private/localhost addresses are always blocked regardless of this flag)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "List the links without opening them")

	return cmd
}

func runConfirm(confirmURL, brokerID string, pending, validateDomain, dryRun bool) error {
	cfg, err := config.Load(resolveConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	activeProfile, err := resolveProfile(cfg)
	if err != nil {
		return err
	}
	brokerDB, err := broker.Load(brokerFile)
	if err != nil {
		return fmt.Errorf("failed to load brokers: %w", err)
	}
	store, err := openHistoryStore(history.DBPathFor(resolveConfigPath()))
	if err != nil {
		return fmt.Errorf("failed to initialize history: %w", err)
	}
	defer func() { _ = store.Close() }()

	var brokerDomains []string
	for _, b := range brokerDB.Brokers {
		brokerDomains = append(brokerDomains, b.Domains()...)
	}
	handler := browser.NewConfirmationHandler(brokerDomains)

	fmt.Println("🔗 Confirmation Link Handler")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	links, err := storedLinksFor(store, activeProfile.ID, "confirmation_required", confirmURL, brokerID, pending,
		func(r history.BrokerResponse) string { return r.ConfirmURL }, "no confirmation URL found for broker")
	if err != nil {
		return err
	}
	if len(links) == 0 {
		fmt.Println("✅ No pending confirmation links")
		return nil
	}

	fmt.Printf("📋 Confirmation links to process: %d\n", len(links))
	if dryRun {
		fmt.Println("🔍 DRY RUN MODE - Links will not be clicked")
	}
	fmt.Println()

	confirmed, failed := 0, 0
	for i, link := range links {
		fmt.Printf("[%d/%d] Processing confirmation link\n", i+1, len(links))
		if link.BrokerID != "" {
			fmt.Printf("       Broker: %s\n", link.BrokerID)
		}
		fmt.Printf("       URL: %s\n", truncate(link.URL, 60))

		switch clickOne(handler, store, activeProfile.ID, link, validateDomain, dryRun) {
		case clickOK:
			confirmed++
		case clickFailed:
			failed++
		}
		fmt.Println()
		if i < len(links)-1 && !dryRun {
			time.Sleep(time.Second) // be gentle with the broker
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if dryRun {
		fmt.Printf("📊 Dry run complete: %d links would be clicked\n", len(links))
	} else {
		fmt.Printf("📊 Complete: %d confirmed, %d failed\n", confirmed, failed)
	}
	return nil
}

type clickOutcome int

const (
	clickOK clickOutcome = iota
	clickFailed
)

// clickOne checks, clicks and reports one link, and records the outcome in the
// pipeline.
func clickOne(handler *browser.ConfirmationHandler, store *history.Store, profileID string, link storedLink, validateDomain, dryRun bool) clickOutcome {
	if validateDomain {
		valid, domain, err := handler.ValidateDomain(link.URL)
		if err != nil {
			fmt.Printf("       ❌ Invalid URL: %v\n", err)
			return clickFailed
		}
		if !valid {
			fmt.Printf("       ⚠️  Domain %s is not a known broker domain\n", domain)
			fmt.Printf("       Use --validate-domain=false to override\n")
			return clickFailed
		}
		fmt.Printf("       ✓ Domain validated: %s\n", domain)
	}
	if dryRun {
		fmt.Printf("       📋 Would click this link (dry run)\n")
		return clickOK
	}

	result, err := handler.ClickConfirmationLink(link.URL, false) // the domain was checked just above
	if err != nil {
		fmt.Printf("       ❌ Error: %v\n", err)
		return clickFailed
	}
	fmt.Printf("       HTTP Status: %d\n", result.StatusCode)
	if len(result.RedirectPath) > 1 {
		fmt.Printf("       Redirects: %d hops\n", len(result.RedirectPath)-1)
	}
	if result.FinalURL != link.URL {
		fmt.Printf("       Final URL: %s\n", truncate(result.FinalURL, 60))
	}

	status := handler.ExtractConfirmationStatus(result)
	stage, outcome, mark := history.PipelineConfirmed, clickOK, "✅"
	if !result.Success {
		stage, outcome, mark = history.PipelineFailed, clickFailed, "⚠️ "
	}
	fmt.Printf("       %s %s\n", mark, status)
	if link.BrokerID != "" {
		_ = store.UpdatePipelineStatus(profileID, link.BrokerID, stage)
	}
	return outcome
}

// truncate shortens s for display to maxLen characters (runes, so a multi-byte
// character is never cut in half), ending in "...".
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-3]) + "..."
}
