package browser

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ConfirmationResult is what came of opening a confirmation link.
type ConfirmationResult struct {
	Success      bool
	URL          string
	FinalURL     string
	StatusCode   int
	ResponseBody string
	ErrorMessage string
	RedirectPath []string
}

// ConfirmationHandler opens the confirmation links brokers email, and only
// those on a known broker domain.
type ConfirmationHandler struct {
	client        *http.Client
	brokerDomains []string
}

const (
	maxRedirects    = 10
	maxConfirmBytes = 64 * 1024
)

var errTooManyRedirects = errors.New("too many redirects")

// NewConfirmationHandler trusts brokerDomains and their subdomains.
func NewConfirmationHandler(brokerDomains []string) *ConfirmationHandler {
	return &ConfirmationHandler{
		client:        &http.Client{Timeout: 30 * time.Second},
		brokerDomains: brokerDomains,
	}
}

// ValidateDomain reports whether the URL is on a known broker domain.
func (h *ConfirmationHandler) ValidateDomain(confirmURL string) (bool, string, error) {
	return matchesAllowedDomain(confirmURL, h.brokerDomains)
}

// ClickConfirmationLink GETs the link as a browser would and judges the page.
// Every redirect hop is checked against the broker domains, validateDomain or
// not, so an open redirect cannot carry the token to a third party.
func (h *ConfirmationHandler) ClickConfirmationLink(confirmURL string, validateDomain bool) (*ConfirmationResult, error) {
	result := &ConfirmationResult{URL: confirmURL, RedirectPath: []string{confirmURL}}
	fail := func(err error, format string, args ...any) (*ConfirmationResult, error) {
		result.ErrorMessage = fmt.Sprintf(format, args...)
		return result, err
	}

	if validateDomain {
		ok, domain, err := h.ValidateDomain(confirmURL)
		if err != nil {
			return fail(err, "%v", err)
		}
		if !ok {
			err := fmt.Errorf("domain %s is not a known broker domain", domain)
			return fail(err, "%v", err)
		}
	}

	req, err := http.NewRequest(http.MethodGet, confirmURL, nil)
	if err != nil {
		return fail(err, "failed to create request: %v", err)
	}
	for k, v := range map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.5",
		"Connection":      "keep-alive",
	} {
		req.Header.Set(k, v)
	}

	// a copy, so concurrent clicks do not share one redirect log
	var hops []string
	client := *h.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errTooManyRedirects
		}
		ok, domain, err := matchesAllowedDomain(next.URL.String(), h.brokerDomains)
		if err != nil {
			return fmt.Errorf("redirect validation failed: %w", err)
		}
		if !ok {
			return fmt.Errorf("redirect to disallowed domain %s blocked", domain)
		}
		hops = append(hops, next.URL.String())
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return fail(err, "request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	result.StatusCode, result.FinalURL = resp.StatusCode, resp.Request.URL.String()
	result.RedirectPath = append(result.RedirectPath, hops...)

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxConfirmBytes))
	if err != nil {
		return fail(err, "failed to read response: %v", err)
	}
	result.ResponseBody = string(body)
	result.Success = confirmationSucceeded(resp.StatusCode, result.ResponseBody)
	if !result.Success {
		result.ErrorMessage = fmt.Sprintf("confirmation may have failed (status %d)", resp.StatusCode)
	}
	return result, nil
}

// what a page says when the link did not work; checked before the success
// phrases, because "failed" and "thank you" can share a page and a wrongly
// confirmed removal is worse than a retried one
var failurePhrases = []string{
	"link expired", "link invalid", "already confirmed", "error occurred",
	"something went wrong", "could not", "unable to", "failed",
}

var successPhrases = []string{
	"successfully", "confirmed", "verification complete", "verified", "opt-out complete",
	"removal complete", "request received", "request confirmed", "thank you", "has been removed",
	"been deleted", "been processed", "unsubscribed", "opted out",
}

// confirmationSucceeded judges a response: a 2xx or 3xx status is needed; a
// failure phrase rules it out; a success phrase confirms it; and a plain 200
// with neither counts as success.
func confirmationSucceeded(status int, body string) bool {
	if status < 200 || status >= 400 {
		return false
	}
	text := strings.ToLower(body)
	if containsAny(text, failurePhrases) {
		return false
	}
	return containsAny(text, successPhrases) || status == http.StatusOK
}

// ExtractConfirmationStatus summarises a failed confirmation for a person.
func (h *ConfirmationHandler) ExtractConfirmationStatus(result *ConfirmationResult) string {
	if result.Success {
		return "Confirmation successful"
	}
	text := strings.ToLower(result.ResponseBody)
	switch {
	case strings.Contains(text, "expired"):
		return "Link expired"
	case strings.Contains(text, "already"):
		return "Already confirmed/processed"
	case strings.Contains(text, "invalid"):
		return "Invalid link"
	case result.StatusCode == http.StatusNotFound:
		return "Link not found (404)"
	case result.StatusCode >= 500:
		return "Server error"
	}
	return "Unknown status"
}
