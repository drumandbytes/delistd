package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// submitSelectors are the usual submit controls, tried in this order.
var submitSelectors = []string{
	"button[type='submit']",
	"input[type='submit']",
	".submit-button",
	"#submit",
	"#submit-btn",
}

// visibleSubmitJS returns the first of the selectors that points at a visible
// element, or "".
const visibleSubmitJS = `(function(selectors) {
	for (var i = 0; i < selectors.length; i++) {
		var el = document.querySelector(selectors[i]);
		if (el && el.offsetParent !== null) return selectors[i];
	}
	return "";
})(%s)`

// clickByTextJS clicks the first button whose label sounds like a submit.
// (CSS has no :contains(), so text matching has to happen in script.)
const clickByTextJS = `(() => {
	const re = /submit|remove|opt.?out|delete|request/i;
	const btn = Array.from(document.querySelectorAll('button, input[type="submit"], input[type="button"]'))
		.find(b => re.test(b.textContent || b.value || ''));
	if (btn) {
		btn.click();
		return true;
	}
	return false;
})()`

// clickTimeout bounds one click, so a control that never becomes clickable
// cannot use up the page's whole time limit.
const clickTimeout = 5 * time.Second

// submitForm submits the form: a visible submit control, else a button whose
// text looks like one, else Enter.
func (b *Browser) submitForm(ctx context.Context) error {
	if selector, err := chromedp.Run(ctx, chromedp.Evaluate[string](fmtSelectors(visibleSubmitJS, submitSelectors))); err == nil && selector != "" {
		clickCtx, cancel := context.WithTimeout(ctx, clickTimeout)
		err := chromedp.Do(clickCtx, chromedp.Click(selector, chromedp.NodeVisible))
		cancel()
		if err == nil {
			return nil
		}
	}

	if clicked, err := chromedp.Run(ctx, chromedp.Evaluate[bool](clickByTextJS)); err == nil && clicked {
		return nil
	}

	if err := chromedp.Do(ctx, chromedp.KeyEvent("\r")); err != nil {
		return errors.New("could not find or click submit button")
	}
	return nil
}

func fmtSelectors(format string, selectors []string) string {
	list, _ := json.Marshal(selectors)
	return fmt.Sprintf(format, list)
}
