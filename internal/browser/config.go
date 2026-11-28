// Package browser fills opt-out forms in a headless Chrome: it finds the
// fields, types the profile in, notices CAPTCHAs, and takes screenshots.
package browser

import "time"

type BrowserConfig struct {
	ExecutablePath string // Chrome or Chromium to launch instead of the one chromedp finds
	RemoteURL      string // attach to a running Chrome's DevTools endpoint instead of launching one; defaults to $DELISTD_CHROME_URL
	Headless       bool
	Timeout        time.Duration
	ScreenshotDir  string
	UserAgent      string
	WindowWidth    int
	WindowHeight   int
	WaitForUser    bool         // pause at a CAPTCHA so a person can solve it
	WaitCallback   func() error // what waiting means, e.g. a prompt in the terminal
}

// DefaultConfig is headless with a one-minute limit per page (many broker
// sites are slow) and a desktop-sized window.
func DefaultConfig() BrowserConfig {
	return BrowserConfig{
		Headless:      true,
		Timeout:       60 * time.Second,
		ScreenshotDir: "",
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		WindowWidth:   1920,
		WindowHeight:  1080,
	}
}

// FormResult is what happened on one page.
type FormResult struct {
	Success         bool
	URL             string
	BrokerID        string
	FieldsFilled    []string
	FieldsMissing   []string
	CaptchaFound    bool
	CaptchaType     string
	ScreenshotPath  string
	ErrorMessage    string
	SubmitAttempted bool
	FillErrors      []string // real failures while filling, such as a timeout; a field that is not on the page is not one
}
