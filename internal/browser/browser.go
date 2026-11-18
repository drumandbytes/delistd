package browser

import (
	"context"
	"fmt"
	"os"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"

	"github.com/drumandbytes/delistd/internal/config"
)

// Browser is one Chrome session, started on first use and kept for every page
// after, as `delistd fill --pending` fills several forms in a row.
type Browser struct {
	config         BrowserConfig
	profile        *config.Profile
	allowedDomains []string

	// the allocator launches or attaches to Chrome; ctx is the browser's own
	// lifetime, which the first action must run on (see start)
	allocCtx    context.Context
	allocCancel context.CancelFunc
	ctx         context.Context
	cancel      context.CancelFunc
}

// extraAllocatorOptions is appended to every launch. Tests use it to run Chrome
// without its sandbox on CI runners that cannot provide one.
var extraAllocatorOptions []chromedp.ExecAllocatorOption

// New prepares a Browser; Chrome itself starts on the first page. A non-empty
// allowedDomains limits the hosts NavigateAndFill visits and fills with personal
// data (matchesAllowedDomain); empty means no check.
func New(cfg BrowserConfig, profile *config.Profile, allowedDomains []string) (*Browser, error) {
	// an explicit Chrome path means "launch that one", whatever the environment says
	if cfg.RemoteURL == "" && cfg.ExecutablePath == "" {
		cfg.RemoteURL = os.Getenv(config.ChromeURLEnv)
	}
	allocCtx, allocCancel := newAllocator(cfg)
	ctx, cancel := chromedp.NewContext(allocCtx)
	return &Browser{
		config: cfg, profile: profile, allowedDomains: allowedDomains,
		allocCtx: allocCtx, allocCancel: allocCancel, ctx: ctx, cancel: cancel,
	}, nil
}

// newAllocator attaches to a remote Chrome (a container, say) or sets up a local
// launch. A remote browser takes no launch flags, so start applies the user agent
// and window size over the protocol instead.
func newAllocator(cfg BrowserConfig) (context.Context, context.CancelFunc) {
	root := context.Background()
	if cfg.RemoteURL != "" {
		return remote.NewAllocator(root, cfg.RemoteURL)
	}
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.WindowSize(cfg.WindowWidth, cfg.WindowHeight),
		chromedp.UserAgent(cfg.UserAgent),
	}
	if cfg.Headless {
		opts = append(opts, chromedp.Headless)
	}
	opts = append(opts, extraAllocatorOptions...) // after the defaults, so tests can override them
	if cfg.ExecutablePath != "" {
		opts = append(opts, chromedp.ExecPath(cfg.ExecutablePath))
	}
	return chromedp.NewExecAllocator(root, opts...)
}

// start connects to or launches Chrome. The first Run's context owns the
// browser, so it has to be b.ctx and not a per-call timeout context, which
// would close Chrome when the call returned. Calling it again is harmless:
// later runs reuse the connection and re-apply the same emulation.
func (b *Browser) start() error {
	var setup []chromedp.Action[chromedp.Void]
	if b.config.RemoteURL != "" {
		setup = append(setup,
			chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
				_, err := cdp.Call(ctx, t, emulation.SetUserAgentOverride, emulation.SetUserAgentOverrideParams{UserAgent: b.config.UserAgent})
				return err
			}),
			chromedp.EmulateViewport(int64(b.config.WindowWidth), int64(b.config.WindowHeight)))
	}
	if err := chromedp.Do(b.ctx, setup...); err != nil {
		if b.config.RemoteURL == "" && config.InContainer() {
			return fmt.Errorf("%w (this image has no browser: set %s to a Chrome DevTools endpoint, see docs/docker.md)", err, config.ChromeURLEnv)
		}
		return err
	}
	return nil
}

// Close shuts Chrome down.
func (b *Browser) Close() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
}
