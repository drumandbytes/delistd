package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/drumandbytes/delistd/internal/country"
	"os"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/email"
	"github.com/drumandbytes/delistd/internal/history"
	"github.com/drumandbytes/delistd/internal/template"
	"github.com/spf13/cobra"
)

var (
	dryRun           bool
	ignoreDailyLimit bool
	resend           bool
	manualSend       bool
	listFlag         string
	brokerIDs        []string
	regions          []string
	categories       []string
	excludedIDs      []string
	statusFilter     string
)

func sendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Email removal requests to data brokers",
		Long: `Send data removal requests to all configured data brokers.

To avoid tripping your email provider's daily sending limit (or looking like
bulk spam to it), each run only sends up to options.daily_send_limit emails
(default 450) and skips brokers it already emailed successfully in the last
25 days. Run it again - the same day or tomorrow - to keep working through
a large broker list; already-sent brokers are automatically skipped, so it's
safe to just re-run 'delistd send' until it reports nothing left to do.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSend()
		},
		Args: cobra.NoArgs,
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be sent, without sending")
	cmd.Flags().BoolVar(&ignoreDailyLimit, "ignore-daily-limit", false, "Send to all matching brokers in one run, ignoring the daily cap (only if your provider can handle the volume)")
	cmd.Flags().BoolVar(&resend, "resend", false, "Also re-send to brokers already emailed within the last 25 days")
	cmd.Flags().BoolVar(&manualSend, "manual", false, "Don't send: show each email and let you mark it sent after you send it by hand (implied by options.send_mode: manual)")
	cmd.Flags().StringVar(&listFlag, "list", "", "Which broker list to use: full (default) or verified (smaller, registry-sourced). Overrides options.broker_list")
	cmd.Flags().StringSliceVar(&brokerIDs, "broker", nil, "Only send to these broker IDs (comma-separated or repeated)")
	addRegionCategoryFlags(cmd, &regions, &categories, "Only send to brokers")
	cmd.Flags().StringSliceVar(&excludedIDs, "exclude", nil, "Skip these broker IDs for this run (comma-separated or repeated)")
	cmd.Flags().StringVar(&statusFilter, "status", "eligible", "History filter: eligible, never, failed, or all")

	return cmd
}

func filterBrokersByStatus(brokers []broker.Broker, statuses map[string]history.BrokerStatus, statusFilter string, now time.Time) []broker.Broker {
	filtered := brokers[:0:0]
	for _, b := range brokers {
		var st *history.BrokerStatus
		if h, ok := statuses[b.ID]; ok {
			st = &h
		}
		if history.MatchesFilter(statusFilter, st, now) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func countWithEmail(brokers []broker.Broker) int {
	n := 0
	for _, b := range brokers {
		if strings.TrimSpace(b.Email) != "" {
			n++
		}
	}
	return n
}

// capSends keeps brokers up to the n-th one with an email. Email-less ones
// never get a history row and sort first every run; they mustn't eat budget.
func capSends(brokers []broker.Broker, n int) []broker.Broker {
	for i, b := range brokers {
		if strings.TrimSpace(b.Email) == "" {
			continue
		}
		if n == 0 {
			return brokers[:i]
		}
		n--
	}
	return brokers
}

// profileBrokers applies the list, region, category, exclusion and country
// rules to the broker database. An empty result has already been explained.
func profileBrokers(cfg *config.Config, activeProfile config.NamedProfile) ([]broker.Broker, error) {
	if listFlag != "" {
		cfg.Options.BrokerList = listFlag
	}
	if cfg.Options.BrokerList != "" && !strings.EqualFold(cfg.Options.BrokerList, "full") && !strings.EqualFold(cfg.Options.BrokerList, "verified") {
		return nil, fmt.Errorf("invalid broker list %q: must be full or verified", cfg.Options.BrokerList)
	}
	brokerDB, err := broker.LoadList(brokerFile, cfg.Options.BrokerFile, cfg.Options.BrokerList)
	if err != nil {
		return nil, fmt.Errorf("failed to load brokers: %w", err)
	}
	if unknown := brokerDB.UnknownIDs(brokerIDs); len(unknown) > 0 {
		return nil, fmt.Errorf("unknown broker ID(s): %s", strings.Join(unknown, ", "))
	}

	selectedRegions := cfg.Options.Regions
	if len(regions) > 0 {
		selectedRegions = regions
	}
	excluded := append(append([]string{}, cfg.Options.ExcludedBrokers...), excludedIDs...)
	brokers := brokerDB.Select(brokerIDs, selectedRegions, categories, excluded, cfg.Options.ExcludedCategories)
	// Naming brokers with --broker is a deliberate choice (e.g. a past
	// address in their country), so only the default selection is scoped.
	if inScope := broker.ForCountry(brokers, activeProfile.Country); len(brokerIDs) == 0 && len(inScope) < len(brokers) {
		fmt.Printf("Skipping %d broker(s) that only handle other countries than %q.\n", len(brokers)-len(inScope), country.Display(activeProfile.Country))
		brokers = inScope
	}
	if len(brokers) == 0 {
		fmt.Println("No brokers to process.")
	}
	return brokers, nil
}

// eligibleBrokers narrows brokers to the --status filter (everything with
// --resend) and orders them by send priority. An empty result has already been
// explained.
func eligibleBrokers(store *history.Store, activeProfile config.NamedProfile, brokers []broker.Broker) ([]broker.Broker, error) {
	statusFilter = strings.ToLower(strings.TrimSpace(statusFilter))
	if statusFilter == "" {
		statusFilter = "eligible"
	}
	if !history.IsSendFilter(statusFilter) {
		return nil, fmt.Errorf("invalid status %q: must be eligible, never, failed, or all", statusFilter)
	}
	if resend {
		statusFilter = "all"
	}
	statuses, err := store.GetAllBrokerStatuses(activeProfile.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check send history: %w", err)
	}
	brokers = filterBrokersByStatus(brokers, statuses, statusFilter, time.Now())
	history.SortBySendPriority(brokers, func(b broker.Broker) time.Time { return statuses[b.ID].LastSent })

	if len(brokers) == 0 {
		fmt.Printf("Nothing to send - no brokers match status %q.\n", statusFilter)
	}
	return brokers, nil
}

// applyDailyCap enforces the rolling daily send cap so a large list cannot blow
// past the provider's per-day limit or read as bulk spam. It returns nil
// brokers when the cap is already used up.
func applyDailyCap(store *history.Store, cfg *config.Config, activeProfile config.NamedProfile, brokers []broker.Broker) ([]broker.Broker, error) {
	// Enforce the rolling daily send cap so a large broker list can't blow
	// past the provider's per-day sending limit or read as bulk-spam
	// behavior. Skipped entirely for --dry-run and --ignore-daily-limit.
	sentLast24h, err := store.CountSentSince(activeProfile.ID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to check daily send count: %w", err)
	}
	budget := cfg.Options.DailySendLimit - sentLast24h
	if budget <= 0 {
		fmt.Printf("📅 Daily send limit reached (%d/%d sent in the last 24h). Run again later, or use --ignore-daily-limit to override.\n",
			sentLast24h, cfg.Options.DailySendLimit)
		return nil, nil
	}
	if sendable := countWithEmail(brokers); budget < sendable {
		fmt.Printf("📅 Daily send limit: sending %d of %d remaining brokers (%d already sent in the last 24h, cap is %d). Re-run later or tomorrow for the rest.\n",
			budget, sendable, sentLast24h, cfg.Options.DailySendLimit)
		brokers = capSends(brokers, budget)
	}
	return brokers, nil
}

// sendLoop emails each broker in turn (or, with --dry-run, only renders the
// email) and prints what happened.
func sendLoop(cfg *config.Config, activeProfile config.NamedProfile, emailCfg config.EmailConfig, brokers []broker.Broker, tmplEngine *template.Engine, sender *email.SMTPSender, store *history.Store) error {
	dry := cfg.Options.DryRun
	var subject string          // of the email a dry run just rendered
	var rateLimit time.Duration // nothing is sent in a dry run, so nothing to pace
	if !dry {
		rateLimit = time.Duration(cfg.Options.RateLimitMs) * time.Millisecond
	}
	batch := &email.Batch{
		Sender: sender, Engine: tmplEngine, Template: cfg.Options.Template, Profile: activeProfile, From: emailCfg.From,
		RateLimit: rateLimit,
		Budget:    -1, // the daily cap already trimmed the list
		Before: func(i int, b broker.Broker, _, _ int) {
			fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(brokers), b.Name, b.Email)
		},
		Skipped: func(i int, b broker.Broker) {
			fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(brokers), b.Name, b.Email)
			if b.OptOutURL != "" {
				fmt.Printf("  ⏭️  No email on file - use the opt-out form instead: %s\n", b.OptOutURL)
			} else {
				fmt.Printf("  ⏭️  No email on file - see notes in brokers.yaml\n")
			}
		},
		After: func(a email.Attempt) {
			switch {
			case a.Err != nil && dry:
				fmt.Printf("  ❌ Failed to render template: %v\n", a.Err)
			case a.Err != nil:
				fmt.Printf("  ❌ %v\n", a.Err)
			case dry:
				fmt.Printf("  📧 Would send: %s\n", subject)
				fmt.Printf("  📍 To: %s\n", a.Broker.Email)
			case a.Record.Status == history.StatusSent:
				fmt.Printf("  ✅ Sent successfully\n")
			default:
				fmt.Printf("  ❌ Failed: %s\n", a.Record.Error)
			}
			if a.RecordErr != nil {
				fmt.Printf("  ⚠️  Failed to record history: %v\n", a.RecordErr)
			}
		},
	}
	if dry {
		batch.Attempt = func(_ context.Context, b broker.Broker) (*history.Record, error) {
			msg, err := tmplEngine.Render(cfg.Options.Template, activeProfile.Profile, b)
			if err != nil {
				return nil, err
			}
			subject = msg.Subject
			return &history.Record{BrokerID: b.ID, Status: history.StatusSent}, nil
		}
	} else {
		batch.Record = store.Add
	}

	res := batch.Run(context.Background(), brokers)
	if res.Stop == email.StopAuth {
		return fmt.Errorf("stopped after %d consecutive authentication failures (%d sent, %d failed) - check your email settings", email.MaxAuthFailures, res.Sent, res.Failed)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if dry {
		fmt.Printf("📊 Dry run complete: %d brokers would receive emails\n", res.Sent)
	} else {
		fmt.Printf("📊 Complete: %d sent, %d failed\n", res.Sent, res.Failed)
	}
	return nil
}

func runSend() error {
	cfg, err := config.Load(resolveConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	manualMode := manualSend || cfg.IsManualSend()

	if err := cfg.Validate(); err != nil && !manualMode {
		return fmt.Errorf("invalid config: %w", err)
	}

	activeProfile, err := resolveProfile(cfg)
	if err != nil {
		return err
	}

	// Override dry-run from command line
	if dryRun {
		cfg.Options.DryRun = true
	}

	brokers, err := profileBrokers(cfg, activeProfile)
	if err != nil || len(brokers) == 0 {
		return err
	}

	// Initialize history store early - needed for both the resend-cooldown
	// skip and the daily send cap below.
	store, err := openHistoryStore(history.DBPathFor(resolveConfigPath()))
	if err != nil {
		return fmt.Errorf("failed to initialize history: %w", err)
	}
	defer func() { _ = store.Close() }()

	brokers, err = eligibleBrokers(store, activeProfile, brokers)
	if err != nil || len(brokers) == 0 {
		return err
	}

	// Manual mode: never touch SMTP. Walk the list, show each email, and
	// record the ones the user confirms they've sent by hand.
	if manualMode && !cfg.Options.DryRun {
		return runSendManual(cfg, activeProfile, brokers, store)
	}

	if !cfg.Options.DryRun && !ignoreDailyLimit {
		brokers, err = applyDailyCap(store, cfg, activeProfile, brokers)
		if err != nil || len(brokers) == 0 {
			return err
		}
	}

	tmplEngine, err := template.NewEngine()
	if err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}
	if err := tmplEngine.UseConfig(cfg); err != nil {
		return fmt.Errorf("custom template: %w", err)
	}

	// Resolve the SMTP account to send from: activeProfile's own mail.email
	// override if it has one, otherwise the shared top-level email: block.
	emailCfg := cfg.EmailForProfile(activeProfile)

	// Initialize email sender (unless dry-run)
	var sender *email.SMTPSender
	if !cfg.Options.DryRun {
		sender = email.NewSMTPSender(emailCfg.SMTP, emailCfg.From)
	}

	if cfg.Options.DryRun {
		fmt.Println("🔍 DRY RUN MODE - No emails will be sent")
		fmt.Println()
	}

	if len(cfg.GetProfiles()) > 1 {
		fmt.Printf("👤 Profile: %s (%s)\n", activeProfile.ID, activeProfile.FullName())
	}
	fmt.Printf("📤 Processing %d brokers...\n", len(brokers))
	fmt.Println()

	return sendLoop(cfg, activeProfile, emailCfg, brokers, tmplEngine, sender, store)
}

// runSendManual walks the broker list, prints each rendered email, and records
// the ones the user says they've sent. No SMTP.
func runSendManual(cfg *config.Config, activeProfile config.NamedProfile, brokers []broker.Broker, store *history.Store) error {
	tmplEngine, err := template.NewEngine()
	if err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}
	if err := tmplEngine.UseConfig(cfg); err != nil {
		return fmt.Errorf("custom template: %w", err)
	}
	tmplName := cfg.Options.Template
	if tmplName == "" {
		tmplName = "gdpr"
	}
	from := cfg.EmailForProfile(activeProfile).From
	if from == "" {
		from = activeProfile.Email
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("✋ Manual mode - Delistd will not send anything.")
	fmt.Println("   For each broker: copy the email into your mail client, send it, then answer.")
	fmt.Printf("   %d broker(s) to review.\n\n", len(brokers))

	recorded, skipped := 0, 0
	// ask puts one question to the user: "yes" records the broker as sent by
	// hand, q stops, anything else moves on.
	ask := func(b broker.Broker, question, yes string) (quit bool, err error) {
		ans := strings.ToLower(prompt(reader, question))
		switch {
		case strings.HasPrefix(ans, "q"):
			return true, nil
		case strings.HasPrefix(ans, yes):
			if err := store.Add(manualSentRecord(activeProfile.ID, b, tmplName)); err != nil {
				return false, fmt.Errorf("failed to record %s: %w", b.ID, err)
			}
			recorded++
		default:
			skipped++
		}
		return false, nil
	}
	for i, b := range brokers {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("[%d/%d] %s (%s)\n\n", i+1, len(brokers), b.Name, b.ID)

		if strings.TrimSpace(b.Email) == "" {
			if b.OptOutURL != "" {
				fmt.Printf("No email on file - use the opt-out form instead:\n  %s\n\n", b.OptOutURL)
			} else {
				fmt.Print("No email on file and no opt-out URL - see notes in brokers.yaml.\n\n")
			}
			quit, err := ask(b, "Mark handled? [y]es / [n]ext / [q]uit: ", "y")
			if err != nil {
				return err
			}
			if quit {
				return finishManual(recorded, skipped)
			}
			continue
		}

		email, err := tmplEngine.Render(tmplName, activeProfile.Profile, b)
		if err != nil {
			fmt.Printf("  ❌ Failed to render: %v\n", err)
			skipped++
			continue
		}
		fmt.Printf("From: %s\nTo: %s\nSubject: %s\n\n%s\n", from, b.Email, email.Subject, email.Body)

		quit, err := ask(b, "Sent it? [s]ent / [n]ext / [q]uit: ", "s")
		if err != nil {
			return err
		}
		if quit {
			return finishManual(recorded, skipped)
		}
	}
	return finishManual(recorded, skipped)
}

func finishManual(recorded, skipped int) error {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Recorded %d as sent, %d left for later.\n", recorded, skipped)
	return nil
}
