package actions

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
)

// Scroll scrolls an element into view.
func Scroll(ctx context.Context, req Request, resolver RefResolver) Response {
	if req.Ref == "" {
		return Response{OK: false, Error: "ref is required for scroll"}
	}

	backendNodeID, err := resolver(req.Ref)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("resolving ref %q: %v", req.Ref, err)}
	}

	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return dom.ScrollIntoViewIfNeeded().WithBackendNodeID(backendNodeID).Do(ctx)
		}),
	)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("scrolling: %v", err)}
	}

	return Response{OK: true, Message: fmt.Sprintf("scrolled %s into view", req.Ref)}
}
