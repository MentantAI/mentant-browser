package actions

import (
	"context"
	"fmt"
	"math"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// Click clicks an element identified by ref.
func Click(ctx context.Context, req Request, resolver RefResolver) Response {
	if req.Ref == "" {
		return Response{OK: false, Error: "ref is required for click"}
	}

	backendNodeID, err := resolver(req.Ref)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("resolving ref %q: %v", req.Ref, err)}
	}

	// Scroll into view first
	_ = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return dom.ScrollIntoViewIfNeeded().WithBackendNodeID(backendNodeID).Do(ctx)
		}),
	)

	// Get coordinates
	x, y, err := getElementCenter(ctx, backendNodeID)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("getting element position: %v", err)}
	}

	// Dispatch mouse events: move, press, release
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := input.DispatchMouseEvent(input.MouseMoved, x, y).Do(ctx); err != nil {
				return err
			}
			if err := input.DispatchMouseEvent(input.MousePressed, x, y).
				WithButton(input.Left).WithClickCount(1).Do(ctx); err != nil {
				return err
			}
			return input.DispatchMouseEvent(input.MouseReleased, x, y).
				WithButton(input.Left).WithClickCount(1).Do(ctx)
		}),
	)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("clicking: %v", err)}
	}

	return Response{OK: true, Message: fmt.Sprintf("clicked %s", req.Ref)}
}

// getElementCenter returns the center coordinates of a backend node.
func getElementCenter(ctx context.Context, backendNodeID cdp.BackendNodeID) (float64, float64, error) {
	var x, y float64
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			model, err := dom.GetBoxModel().WithBackendNodeID(backendNodeID).Do(ctx)
			if err != nil {
				return fmt.Errorf("getBoxModel: %w", err)
			}
			if model.Content == nil || len(model.Content) < 8 {
				return fmt.Errorf("no content quad for node")
			}
			quad := model.Content
			x = (quad[0] + quad[2] + quad[4] + quad[6]) / 4
			y = (quad[1] + quad[3] + quad[5] + quad[7]) / 4

			if math.IsNaN(x) || math.IsNaN(y) {
				return fmt.Errorf("invalid coordinates")
			}
			return nil
		}),
	)
	return x, y, err
}
