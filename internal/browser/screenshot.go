package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

func (b *Browser) takeScreenshot(ctx context.Context, brokerID, suffix string) (string, error) {
	return b.takeScreenshotQuality(ctx, brokerID, suffix, 90)
}

// takeScreenshotQuality saves the whole page and returns the file name. At
// quality 100 Chrome returns a PNG and anything lower is a JPEG, so the file
// extension follows.
func (b *Browser) takeScreenshotQuality(ctx context.Context, brokerID, suffix string, quality int) (string, error) {
	img, err := chromedp.Run(ctx, chromedp.FullScreenshot(quality))
	if err != nil {
		return "", err
	}
	// owner-only: a screenshot can hold personal data typed mid-fill
	if err := os.MkdirAll(b.config.ScreenshotDir, 0o700); err != nil {
		return "", err
	}
	ext := "jpg"
	if quality >= 100 {
		ext = "png"
	}
	name := fmt.Sprintf("%s_%s_%d.%s", brokerID, suffix, time.Now().Unix(), ext)
	if err := os.WriteFile(filepath.Join(b.config.ScreenshotDir, name), img, 0o600); err != nil {
		return "", err
	}
	return name, nil
}
