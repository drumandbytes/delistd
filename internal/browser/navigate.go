package browser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	dynamicContentWait = 2 * time.Second // most a page gets to build itself after loading
	afterFillWait      = time.Second
	afterSubmitWait    = 2 * time.Second
)

// NavigateAndFill opens url, fills the form it finds with the profile and, if
// autoSubmit, submits it. It stops for a CAPTCHA: a blocking one before the
// form, or one on the form itself, which is handled by waiting for a person
// when WaitForUser is set and otherwise reported in the result.
func (b *Browser) NavigateAndFill(url string, brokerID string, autoSubmit bool) (*FormResult, error) {
	result := &FormResult{URL: url, BrokerID: brokerID}

	// Form URLs can come from untrusted email, so personal data only goes to
	// known broker domains. An empty list means none is configured.
	if len(b.allowedDomains) > 0 {
		ok, domain, err := matchesAllowedDomain(url, b.allowedDomains)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("invalid URL: %v", err)
			return result, err
		}
		if !ok {
			err := fmt.Errorf("URL domain not in known broker list: %s", domain)
			result.ErrorMessage = err.Error()
			return result, err
		}
	}

	// Chrome starts on b.ctx, not on the per-call timeout context below,
	// which would close it after one form.
	if err := b.start(); err != nil {
		result.ErrorMessage = fmt.Sprintf("browser failed to start: %v", err)
		return result, err
	}
	ctx, cancel := context.WithTimeout(b.ctx, b.config.Timeout)
	defer cancel()

	if err := chromedp.Do(ctx, chromedp.Navigate(url)); err != nil {
		result.ErrorMessage = fmt.Sprintf("navigation failed: %v", err)
		return result, err
	}
	if err := chromedp.Do(ctx, chromedp.WaitReady("body")); err != nil {
		result.ErrorMessage = fmt.Sprintf("page load failed: %v", err)
		return result, err
	}
	b.settle(ctx, dynamicContentWait)

	// A CAPTCHA can sit in front of the form (a Cloudflare challenge, say).
	gate, err := b.detectCaptcha(ctx)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("captcha detection failed: %v", err)
		return result, err
	}
	if gate.IsCaptchaBlocking() {
		result.CaptchaFound, result.CaptchaType = true, gate.Type
		b.shoot(ctx, result, brokerID, "captcha_gate")
		if !b.canWaitForUser() {
			result.ErrorMessage = fmt.Sprintf("Blocking CAPTCHA detected: %s - use --wait flag to solve manually", gate.Type)
			return result, nil
		}
		if err := b.waitForUser(result, "Blocking CAPTCHA detected: "+gate.GetCaptchaDescription(), "Please solve the CAPTCHA in the browser window..."); err != nil {
			return result, err
		}
		b.settle(ctx, dynamicContentWait) // the page reacts to the solved CAPTCHA
	}

	fill := NewFormFiller(b.profile).Fill(ctx)
	result.FieldsFilled, result.FieldsMissing, result.FillErrors = fill.FilledFields, fill.MissingFields, fill.Errors
	b.settle(ctx, afterFillWait)

	// ... or on the form itself (reCAPTCHA next to the submit button)
	onForm, err := b.detectCaptcha(ctx)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("captcha detection failed: %v", err)
		return result, err
	}
	if onForm.IsCaptchaBlocking() {
		result.CaptchaFound, result.CaptchaType = true, onForm.Type
		b.shoot(ctx, result, brokerID, "filled_captcha") // the filled form and the CAPTCHA, for a person to review
		if !b.canWaitForUser() {
			result.ErrorMessage = fmt.Sprintf("CAPTCHA detected: %s - form filled, solve CAPTCHA and submit", onForm.Type)
			return result, nil
		}
		if err := b.waitForUser(result, "Form CAPTCHA detected: "+onForm.GetCaptchaDescription(), "Form has been pre-filled. Please solve the CAPTCHA..."); err != nil {
			return result, err
		}
		if autoSubmit {
			b.submit(ctx, result, brokerID, "submit failed after CAPTCHA")
		}
		return result, nil
	}

	b.shoot(ctx, result, brokerID, "filled")

	filledSomething := len(result.FieldsFilled) > 0 && len(result.FillErrors) == 0
	switch {
	case autoSubmit && filledSomething && !result.CaptchaFound: // a CAPTCHA a person solved is left for them to submit
		b.submit(ctx, result, brokerID, "submit failed")
	case filledSomething:
		result.Success = true
	}
	// a real error while filling (the browser died mid-fill, say) must not be
	// hidden behind Success just because another field went in first
	if len(result.FillErrors) > 0 && result.ErrorMessage == "" {
		result.ErrorMessage = fmt.Sprintf("field fill errors: %s", strings.Join(result.FillErrors, "; "))
	}
	return result, nil
}

func (b *Browser) canWaitForUser() bool { return b.config.WaitForUser && b.config.WaitCallback != nil }

// waitForUser tells the person what is blocking and blocks in the callback
// until they say they are done.
func (b *Browser) waitForUser(result *FormResult, what, ask string) error {
	fmt.Printf("⚠️  %s\n", what)
	fmt.Printf("   %s\n", ask)
	if err := b.config.WaitCallback(); err != nil {
		result.ErrorMessage = fmt.Sprintf("user cancelled: %v", err)
		return err
	}
	return nil
}

// shoot saves a screenshot when a directory is configured and keeps its name
// on the result.
func (b *Browser) shoot(ctx context.Context, result *FormResult, brokerID, suffix string) {
	if b.config.ScreenshotDir == "" {
		return
	}
	result.ScreenshotPath, _ = b.takeScreenshot(ctx, brokerID, suffix)
}

// submit clicks the form's submit control and notes the outcome; failure is
// recorded in the result's message, with prefix saying which submit it was.
func (b *Browser) submit(ctx context.Context, result *FormResult, brokerID, prefix string) {
	if err := b.submitForm(ctx); err != nil {
		result.ErrorMessage = fmt.Sprintf("%s: %v", prefix, err)
		return
	}
	result.SubmitAttempted, result.Success = true, true
	b.settle(ctx, afterSubmitWait)
	if b.config.ScreenshotDir != "" {
		_, _ = b.takeScreenshot(ctx, brokerID, "submitted")
	}
}
