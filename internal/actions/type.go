package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// Type types text character-by-character using keyboard events.
// If ref is provided, clicks the element first to focus it.
func Type(ctx context.Context, req Request, resolver RefResolver) Response {
	text := req.Text
	if text == "" {
		text = req.Value
	}
	if text == "" {
		return Response{OK: false, Error: "text is required for type"}
	}

	// Click to focus if ref provided
	if req.Ref != "" {
		result := Click(ctx, Request{Kind: "click", Ref: req.Ref}, resolver)
		if !result.OK {
			return result
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Type each character
	for _, ch := range text {
		err := chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				return input.DispatchKeyEvent(input.KeyChar).WithText(string(ch)).Do(ctx)
			}),
		)
		if err != nil {
			return Response{OK: false, Error: fmt.Sprintf("typing character: %v", err)}
		}
	}

	return Response{OK: true, Message: fmt.Sprintf("typed %d characters", len(text))}
}
