package main

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/drumandbytes/delistd/internal/broker"
	"github.com/drumandbytes/delistd/internal/config"
	"github.com/drumandbytes/delistd/internal/schedule"
	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	brokerFile  string
	profileFlag string
)

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "dev"

// resolveProfile applies --profile; optional with a single profile, required
// with several.
func resolveProfile(cfg *config.Config) (config.NamedProfile, error) {
	return cfg.GetProfile(profileFlag)
}

// resolveBrokerWritePath returns a filesystem path for the few commands that
// modify the broker database (add-broker, cleanup-bounces). Read-only commands
// use broker.Load(brokerFile), which also has an embedded fallback. Order:
// --brokers, then a local ./data/brokers.yaml checkout, then the per-user copy.
func resolveBrokerWritePath() string {
	if p, ok := listFileWritePath(); ok {
		return p
	}
	return broker.UserBrokersPath()
}

// listFileWritePath is the broker list file to edit in place when the user is
// maintaining a list - an explicit --brokers file or a source checkout's
// data/brokers.yaml. ok=false for an installed copy, whose own additions and
// corrections belong in broker.LocalPath (update-brokers replaces the list).
func listFileWritePath() (string, bool) {
	const checkout = "data/brokers.yaml"
	switch _, err := os.Stat(checkout); {
	case brokerFile != "":
		return brokerFile, true
	case err == nil:
		return checkout, true
	}
	return "", false
}

// resolveConfigPath is --config, else the default location.
func resolveConfigPath() string {
	return cmp.Or(cfgFile, config.DefaultConfigPath())
}

// newRootCmd builds the CLI; each call binds the flag globals afresh, so
// tests can run several commands in one process.
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "delistd",
		Version: version,
		Short:   "Delistd - Automated data broker removal requests",
		Long: `Delistd is open-source software that asks data brokers to delete what they
hold about you, by email from your own account.

It has templates for GDPR, CCPA and a generic request, and works with any
SMTP provider (Gmail, Proton Mail Bridge, Fastmail and others).`,
	}
	// One-time note when a pre-3.0 OS schedule is still installed (schedule/legacy.go).
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		for c := cmd; c != nil; c = c.Parent() {
			if c.Name() == "schedule" { // `schedule install|remove|status` deal with it themselves
				return
			}
		}
		if msg := schedule.LegacyNoticeOnce(filepath.Dir(resolveConfigPath())); msg != "" {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "NOTE:", msg)
		}
	}
	// Bare `delistd` (e.g. double-clicking the binary) starts the web UI, which
	// opens the browser. Subcommands and --help behave as before.
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runBare(serveFn, runtime.GOOS, os.Stdin, os.Stderr)
	}
	// So a standalone binary (no LICENSE file alongside it) still points at
	// the MIT terms it ships under.
	rootCmd.SetVersionTemplate("delistd {{.Version}}\nMIT licensed - https://github.com/drumandbytes/delistd/blob/main/LICENSE\n")

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.delistd/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&brokerFile, "brokers", "", "broker list file (default: ~/.delistd/brokers.yaml if `update-brokers` has written one, else the list built into the binary)")
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "Profile ID to operate as (default: the only configured profile; required if you've configured more than one via 'delistd profile add')")

	for _, build := range []func() *cobra.Command{
		initCmd, sendCmd, serveCmd, mcpCmd, listBrokersCmd, statusCmd, addBrokerCmd,
		monitorCmd, pipelineCmd, fillCmd, confirmCmd, cleanupBouncesCmd, markBouncedCmd,
		auditBrokersCmd, profileCmd, exportCmd, draftCmd, markSentCmd, updateBrokersCmd,
		validateBrokersCmd, guidesCmd, healthcheckCmd, autoCmd, scheduleCmd, passphraseCmd, oauthCmd,
	} {
		rootCmd.AddCommand(build())
	}
	return rootCmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// prompt prints message and returns the trimmed line read, or "" at end of input.
func prompt(reader *bufio.Reader, message string) string {
	fmt.Print(message)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// serveFn starts the web UI; tests replace it.
var serveFn = runServe

// runBare is the no-argument launch. On Windows a startup error waits for
// Enter, because a double-clicked console closes at once.
func runBare(serve func(port int, listen string) error, goos string, in io.Reader, errOut io.Writer) error {
	// the bare launch has no --listen: always loopback
	err := serve(8080, "127.0.0.1")
	if err != nil && goos == "windows" {
		_, _ = fmt.Fprintln(errOut, "Delistd could not start:", err)
		_, _ = fmt.Fprint(errOut, "Press Enter to close.")
		_, _ = bufio.NewReader(in).ReadString('\n')
	}
	return err
}
