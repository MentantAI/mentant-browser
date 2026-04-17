package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

// Wait polls the accessibility tree until an element matching the criteria appears.
func Wait(ctx context.Context, req Request, resolver RefResolver) Response {
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10
	}

	text := req.Text
	if text == "" {
		text = req.Value
	}
	if text == "" && req.Ref == "" {
		return Response{OK: false, Error: "text or ref is required for wait"}
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	for time.Now().Before(deadline) {
		found, err := checkForElement(ctx, text, req.Ref)
		if err == nil && found {
			return Response{OK: true, Message: fmt.Sprintf("found element matching %q", text)}
		}
		time.Sleep(1 * time.Second)
	}

	return Response{OK: false, Error: fmt.Sprintf("timeout after %ds waiting for %q", timeout, text)}
}

func checkForElement(ctx context.Context, text, ref string) (bool, error) {
	var nodes []*accessibility.Node
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			tree, err := accessibility.GetFullAXTree().Do(ctx)
			if err != nil {
				return err
			}
			nodes = tree
			return nil
		}),
	)
	if err != nil {
		return false, err
	}

	for _, n := range nodes {
		if text != "" && n.Name != nil {
			var name string
			if err := json.Unmarshal(n.Name.Value, &name); err == nil {
				if strings.Contains(strings.ToLower(name), strings.ToLower(text)) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
