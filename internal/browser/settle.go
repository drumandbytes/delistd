package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// settleJS resolves once the page has stopped changing for `quiet`
// milliseconds, or after `limit`.
const settleJS = `new Promise(function(resolve) {
	var timer;
	var done = function() { observer.disconnect(); resolve(true); };
	var observer = new MutationObserver(function() { clearTimeout(timer); timer = setTimeout(done, %d); });
	observer.observe(document, {childList: true, subtree: true, attributes: true, characterData: true});
	timer = setTimeout(done, %d);
	setTimeout(done, %d);
})`

// quietPeriod is how long the page must hold still to count as settled.
const quietPeriod = 700 * time.Millisecond

// settle waits for the page to stop changing, at most limit. Scripts and
// late requests that fill in a form or a result page used to be waited for
// with a flat sleep of the full limit; most pages are quiet well before it.
// Whatever the page does (it may navigate away mid-wait), settle never waits
// past limit.
func (b *Browser) settle(ctx context.Context, limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		left := time.Until(deadline)
		quiet := min(quietPeriod, left)
		js := formatSettle(quiet, left)
		_, err := chromedp.Run(ctx, chromedp.Evaluate[bool](js, func(p *runtime.EvaluateParams) { p.AwaitPromise = new(true) }))
		if err == nil || isContextErr(err) {
			return
		}
		// the document was replaced under us (a navigation): look again
		select {
		case <-ctx.Done():
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func formatSettle(quiet, limit time.Duration) string {
	return fmt.Sprintf(settleJS, quiet.Milliseconds(), quiet.Milliseconds(), limit.Milliseconds())
}
