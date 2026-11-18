package browser

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// DELISTD_CHROME_URL path: Delistd attaches to a Chrome somebody else started
// (the compose sidecar) and applies the user agent and window over CDP.
func TestRemoteChromeIsDrivenOverCDP(t *testing.T) {
	requireChrome(t).Close() // skips when there is no browser

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	opts := append([]chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck, chromedp.DisableGPU, chromedp.Headless,
		remote.WebSocket, chromedp.Flag("remote-debugging-port", strconv.Itoa(port)),
	}, extraAllocatorOptions...)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()
	host, cancelHost := chromedp.NewContext(allocCtx)
	defer cancelHost()
	if err := chromedp.Do(host); err != nil { // start the "sidecar"
		t.Skipf("could not start a Chrome with a debugging port: %v", err)
	}

	cfg := DefaultConfig()
	cfg.RemoteURL = "http://127.0.0.1:" + strconv.Itoa(port)
	cfg.UserAgent = "delistd-remote-test"
	b, err := New(cfg, testProfile(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := b.start(); err != nil {
		t.Fatalf("start against the remote Chrome: %v", err)
	}
	if err := chromedp.Do(b.ctx, chromedp.Navigate("data:text/html,<title>remote</title>")); err != nil {
		t.Fatal(err)
	}
	title, err := chromedp.Run(b.ctx, chromedp.Title())
	if err != nil || title != "remote" {
		t.Fatalf("title = %q, %v", title, err)
	}
	ua, err := chromedp.Run(b.ctx, chromedp.Evaluate[string]("navigator.userAgent"))
	if err != nil || ua != "delistd-remote-test" {
		t.Errorf("user agent = %q, %v; start() should override it over CDP", ua, err)
	}
}
