package screenshot

import (
	"context"
	"encoding/base64"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Result holds the screenshot data.
type Result struct {
	Data   string `json:"data"`   // base64-encoded PNG
	Format string `json:"format"` // "png"
}

// Take captures a screenshot of the current page.
func Take(ctx context.Context, fullPage bool) (*Result, error) {
	var buf []byte

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			params := page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatPng)
			if fullPage {
				params = params.WithCaptureBeyondViewport(true)

				// Get full page dimensions
				_, _, contentSize, _, _, _, err := page.GetLayoutMetrics().Do(ctx)
				if err == nil && contentSize != nil {
					params = params.WithClip(&page.Viewport{
						X:      0,
						Y:      0,
						Width:  contentSize.Width,
						Height: contentSize.Height,
						Scale:  1,
					})
				}
			}

			var err error
			buf, err = params.Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, err
	}

	return &Result{
		Data:   base64.StdEncoding.EncodeToString(buf),
		Format: "png",
	}, nil
}
