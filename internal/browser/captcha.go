package browser

import (
	"context"

	"github.com/chromedp/chromedp"
)

// The kinds of CAPTCHA detectCaptcha tells apart. The strings are what the page
// script returns.
const (
	CaptchaTypeRecaptchaV2  = "recaptcha_v2"
	CaptchaTypeRecaptchaV3  = "recaptcha_v3"
	CaptchaTypeHCaptcha     = "hcaptcha"
	CaptchaTypeTurnstile    = "cloudflare_turnstile"
	CaptchaTypeFunCaptcha   = "funcaptcha"
	CaptchaTypeCloudflare   = "cloudflare_challenge"
	CaptchaTypeImageCaptcha = "image_captcha"
	CaptchaTypeTextCaptcha  = "text_captcha"
	CaptchaTypeUnknown      = "unknown"
)

// CaptchaInfo says whether a page has a CAPTCHA and which kind.
type CaptchaInfo struct {
	Found bool
	Type  string // a CaptchaType constant
}

// detectCaptchaJS evaluates to the first CAPTCHA kind found on the page, the
// most specific checks first, or "" when there is none.
const detectCaptchaJS = `(function() {
	const has = (sel) => document.querySelector(sel) !== null;
	const text = document.body ? document.body.innerText.toLowerCase() : '';
	const title = document.title.toLowerCase();

	if (has('iframe[src*="recaptcha"]') || has('.g-recaptcha, [data-sitekey]') || typeof grecaptcha !== 'undefined') return 'recaptcha_v2';
	const v3Script = Array.from(document.querySelectorAll('script[src*="recaptcha"]')).some((s) => s.src.includes('render='));
	if (v3Script || has('script[src*="enterprise.js"]')) return 'recaptcha_v3';
	if (has('iframe[src*="hcaptcha"]') || has('.h-captcha, [data-hcaptcha-sitekey]') || typeof hcaptcha !== 'undefined') return 'hcaptcha';
	if (has('iframe[src*="challenges.cloudflare.com"]') || has('.cf-turnstile, [data-turnstile-sitekey]')) return 'cloudflare_turnstile';
	if (has('iframe[src*="funcaptcha"], iframe[src*="arkoselabs"]')) return 'funcaptcha';
	const fun = document.querySelector('#FunCaptcha, [data-callback]');
	if (fun && fun.id && fun.id.toLowerCase().includes('funcaptcha')) return 'funcaptcha';

	const cfTitle = title.includes('just a moment') || title.includes('checking your browser');
	if (cfTitle || has('form#challenge-form') || has('.ray-id, [data-ray]') || (text.includes('ray id') && text.includes('cloudflare'))) return 'cloudflare_challenge';

	const phrases = ['captcha', 'verification code', 'security code', 'enter the code', 'type the characters',
		'verify you are human', 'prove you are human', 'i am not a robot', 'human verification'];
	const hasImage = has('img[src*="captcha"], img[alt*="captcha"], .captcha-image');
	const hasInput = has('input[name*="captcha"], input[id*="captcha"], input[placeholder*="captcha" i]');
	if ((hasImage || hasInput) && phrases.some((p) => text.includes(p))) return hasInput ? 'text_captcha' : 'unknown';
	return has('img[src*="captcha"], img[alt*="captcha"]') ? 'image_captcha' : '';
})()`

// detectCaptcha errors only when the context fails (the browser died or timed
// out); a page whose script throws counts as having no CAPTCHA.
func (b *Browser) detectCaptcha(ctx context.Context) (CaptchaInfo, error) {
	kind, err := chromedp.Run(ctx, chromedp.Evaluate[string](detectCaptchaJS))
	switch {
	case err != nil && isContextErr(err):
		return CaptchaInfo{}, err
	case err != nil:
		return CaptchaInfo{}, nil
	}
	return CaptchaInfo{Found: kind != "", Type: kind}, nil
}

// IsCaptchaBlocking reports whether the CAPTCHA needs a person. reCAPTCHA v3 is
// invisible and normally doesn't.
func (c CaptchaInfo) IsCaptchaBlocking() bool {
	return c.Found && c.Type != CaptchaTypeRecaptchaV3
}

// GetCaptchaDescription says what the CAPTCHA is and what solving it takes.
func (c CaptchaInfo) GetCaptchaDescription() string {
	if !c.Found {
		return "No CAPTCHA detected"
	}
	switch c.Type {
	case CaptchaTypeRecaptchaV2:
		return "Google reCAPTCHA v2: tick the box and/or solve the image puzzle"
	case CaptchaTypeRecaptchaV3:
		return "Google reCAPTCHA v3: normally invisible, may pass by itself"
	case CaptchaTypeHCaptcha:
		return "hCaptcha: pick the images that match the prompt"
	case CaptchaTypeTurnstile:
		return "Cloudflare Turnstile: normally passes after a short check"
	case CaptchaTypeFunCaptcha:
		return "FunCaptcha: finish the interactive puzzle"
	case CaptchaTypeCloudflare:
		return "Cloudflare challenge: wait for it or complete the verification"
	case CaptchaTypeImageCaptcha:
		return "Image CAPTCHA: type the characters shown"
	case CaptchaTypeTextCaptcha:
		return "Text CAPTCHA: enter the verification code"
	}
	return "Unknown CAPTCHA type: needs a manual look"
}
