package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/drumandbytes/delistd/internal/country"
	"github.com/drumandbytes/delistd/internal/datadir"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/browser"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/history"
	"github.com/spf13/cobra"
)

// formBrowser is what runFill needs from browser.Browser; tests stub it so
// the CLI's result handling runs without Chrome.
type formBrowser interface {
	NavigateAndFill(url, brokerID string, autoSubmit bool) (*browser.FormResult, error)
	Close()
}

var newFormBrowser = func(cfg browser.BrowserConfig, p *config.Profile, domains []string) (formBrowser, error) {
	return browser.New(cfg, p, domains)
}

func fillCmd() *cobra.Command {
	var brokerID string
	var formURL string
	var headless bool
	var autoSubmit bool
	var screenshotDir string
	var pending bool
	var waitForCaptcha bool

	cmd := &cobra.Command{
		Use:   "fill",
		Short: "Fill in broker opt-out forms in a browser",
		Long: `Opens a broker's opt-out form in Chrome and types your profile into it.

For each form it:
- finds the fields and enters what your profile holds for them
- notices a CAPTCHA and turns it into a task for you to solve
- submits the form only if you ask it to (--submit)

Examples:
  # Fill a specific form URL
  delistd fill --url "https://example.com/optout"

  # Fill form for a specific broker (using URL from pipeline)
  delistd fill --broker spokeo

  # Fill all pending forms from the pipeline
  delistd fill --pending

  # Fill with visible browser window (for debugging)
  delistd fill --url "https://example.com/optout" --headless=false

  # Fill form and wait for you to solve CAPTCHA, then auto-submit
  delistd fill --url "https://example.com/optout" --headless=false --wait --submit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			headlessFlagSet := cmd.Flags().Changed("headless")
			return runFill(brokerID, formURL, headless, headlessFlagSet, autoSubmit, screenshotDir, pending, waitForCaptcha)
		},
	}

	cmd.Flags().StringVar(&brokerID, "broker", "", "Fill the form a broker's reply pointed to (by broker ID)")
	cmd.Flags().StringVar(&formURL, "url", "", "The opt-out form's address")
	cmd.Flags().BoolVar(&headless, "headless", true, "Run Chrome without a window (default: pipeline.browser_headless in your config, else yes)")
	cmd.Flags().BoolVar(&autoSubmit, "submit", false, "Submit the form once it is filled in")
	cmd.Flags().StringVar(&screenshotDir, "screenshots", "", "Where screenshots are saved (default: screenshots in the data directory)")
	cmd.Flags().BoolVar(&pending, "pending", false, "Fill every form the pipeline is waiting on")
	cmd.Flags().BoolVar(&waitForCaptcha, "wait", false, "Pause at a CAPTCHA until you have solved it (needs --headless=false)")

	return cmd
}

func runFill(brokerID, formURL string, headless, headlessFlagSet, autoSubmit bool, screenshotDir string, pending bool, waitForCaptcha bool) error {
	cfg, err := config.Load(resolveConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	activeProfile, err := resolveProfile(cfg)
	if err != nil {
		return err
	}
	// NavigateAndFill types personal data into whatever page it is given and
	// form URLs can come from email (untrusted), so it only visits known
	// broker domains.
	brokerDB, err := broker.Load(brokerFile)
	if err != nil {
		return fmt.Errorf("failed to load brokers: %w", err)
	}
	var brokerDomains []string
	for _, b := range brokerDB.Brokers {
		brokerDomains = append(brokerDomains, b.Domains()...)
	}

	// --headless wins when given; otherwise the config's pipeline.browser_headless
	if !headlessFlagSet {
		headless = cfg.Pipeline.Headless()
	}
	if screenshotDir == "" {
		home, _ := os.UserHomeDir()
		screenshotDir = filepath.Join(datadir.Of(home), "screenshots")
	}
	store, err := openHistoryStore(history.DBPathFor(resolveConfigPath()))
	if err != nil {
		return fmt.Errorf("failed to initialize history: %w", err)
	}
	defer func() { _ = store.Close() }()

	browserCfg := browser.DefaultConfig()
	browserCfg.Headless = headless
	browserCfg.ScreenshotDir = screenshotDir
	if cfg.Pipeline.BrowserTimeoutSec > 0 {
		browserCfg.Timeout = time.Duration(cfg.Pipeline.BrowserTimeoutSec) * time.Second
	}
	if waitForCaptcha {
		if headless {
			fmt.Println("⚠️  Warning: --wait requires --headless=false to be useful")
		}
		browserCfg.WaitForUser = true
		browserCfg.Timeout = 5 * time.Minute // long enough for a person to solve it
		browserCfg.WaitCallback = waitForEnter
	}

	b, err := newFormBrowser(browserCfg, &activeProfile.Profile, brokerDomains)
	if err != nil {
		return fmt.Errorf("failed to create browser: %w", err)
	}
	defer b.Close()

	fmt.Println("🌐 Browser Automation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	forms, err := storedLinksFor(store, activeProfile.ID, "form_required", formURL, brokerID, pending,
		func(r history.BrokerResponse) string { return r.FormURL }, "no form URL found for broker")
	if err != nil {
		return err
	}
	if len(forms) == 0 {
		fmt.Println("✅ No pending forms to fill")
		return nil
	}

	fmt.Printf("📋 Forms to process: %d\n", len(forms))
	fmt.Println()
	for i, form := range forms {
		fmt.Printf("[%d/%d] Processing %s\n", i+1, len(forms), form.URL)
		if form.BrokerID != "" {
			fmt.Printf("       Broker: %s\n", form.BrokerID)
		}
		result, err := b.NavigateAndFill(form.URL, form.BrokerID, autoSubmit)
		if err != nil {
			fmt.Printf("       ❌ Error: %v\n", err)
			continue
		}
		reportFill(store, brokerDB, activeProfile, form, result)
		fmt.Println()
		if i < len(forms)-1 {
			time.Sleep(2 * time.Second) // be gentle with the brokers
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("✅ Processed %d forms\n", len(forms))
	return nil
}

// waitForEnter is what --wait does at a CAPTCHA: ask, then block until Enter.
func waitForEnter() error {
	fmt.Println()
	fmt.Println("       ⏸️  CAPTCHA detected! Solve it in the browser window.")
	fmt.Println("       Press ENTER when done (or Ctrl+C to cancel)...")
	fmt.Println()
	_, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return err
}

// reportFill prints what a fill did and records it: a CAPTCHA becomes a task
// for a person, a filled or submitted form moves the pipeline stage.
func reportFill(store *history.Store, db *broker.BrokerDatabase, profile config.NamedProfile, form storedLink, result *browser.FormResult) {
	if len(result.FieldsFilled) > 0 {
		fmt.Printf("       ✅ Filled fields: %s\n", strings.Join(result.FieldsFilled, ", "))
	}
	if len(result.FieldsMissing) > 0 {
		fmt.Printf("       ⚠️  Missing profile data for: %s\n", strings.Join(result.FieldsMissing, ", "))
	}

	switch {
	case result.CaptchaFound:
		fmt.Printf("       🤖 CAPTCHA detected: %s\n", result.CaptchaType)
		// the helper page copies these into the form by hand
		profileJSON, _ := json.Marshal(map[string]string{
			"email": profile.Email, "firstName": profile.FirstName, "lastName": profile.LastName,
			"phone": profile.Phone, "address": profile.Address, "city": profile.City,
			"state": profile.State, "zipCode": profile.ZipCode, "country": country.Display(profile.Country),
		})
		name := form.BrokerID
		if b := db.FindByID(form.BrokerID); b != nil {
			name = b.Name
		}
		task := &history.PendingTask{
			ProfileID: profile.ID, BrokerID: form.BrokerID, BrokerName: name, TaskType: history.TaskCaptcha,
			FormURL: form.URL, BrowserState: string(profileJSON), Status: "pending", ScreenshotPath: result.ScreenshotPath,
		}
		if err := store.AddPendingTask(task); err != nil {
			fmt.Printf("       ⚠️  Failed to create task: %v\n", err)
		} else {
			fmt.Printf("       📝 Created CAPTCHA task for manual solving\n")
		}
		_ = store.UpdatePipelineStatus(profile.ID, form.BrokerID, history.PipelineAwaitingCaptcha)
	case result.SubmitAttempted:
		fmt.Printf("       📨 Form submitted!\n")
		_ = store.UpdatePipelineStatus(profile.ID, form.BrokerID, history.PipelineFormFilled)
	case result.Success:
		fmt.Printf("       ✅ Form filled (not submitted)\n")
		_ = store.UpdatePipelineStatus(profile.ID, form.BrokerID, history.PipelineFormFilled)
	}
	if result.ScreenshotPath != "" {
		fmt.Printf("       📸 Screenshot: %s\n", result.ScreenshotPath)
	}
}
