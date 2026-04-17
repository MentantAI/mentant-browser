package actions

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Select selects an option in a dropdown by value or text.
func Select(ctx context.Context, req Request, resolver RefResolver) Response {
	if req.Ref == "" {
		return Response{OK: false, Error: "ref is required for select"}
	}
	if req.Value == "" {
		return Response{OK: false, Error: "value is required for select"}
	}

	backendNodeID, err := resolver(req.Ref)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("resolving ref %q: %v", req.Ref, err)}
	}

	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			obj, err := dom.ResolveNode().WithBackendNodeID(backendNodeID).Do(ctx)
			if err != nil {
				return err
			}

			script := `
				function(value) {
					const options = Array.from(this.options || []);
					const opt = options.find(o => o.value === value || o.textContent.trim() === value);
					if (opt) {
						this.value = opt.value;
						this.dispatchEvent(new Event('change', { bubbles: true }));
						return true;
					}
					return false;
				}
			`

			result, _, err := runtime.CallFunctionOn(script).
				WithObjectID(obj.ObjectID).
				WithArguments([]*runtime.CallArgument{{Value: []byte(fmt.Sprintf("%q", req.Value))}}).
				WithReturnByValue(true).
				Do(ctx)
			if err != nil {
				return err
			}
			if string(result.Value) == "false" {
				return fmt.Errorf("option %q not found in select", req.Value)
			}
			return nil
		}),
	)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("selecting: %v", err)}
	}

	return Response{OK: true, Message: fmt.Sprintf("selected %q in %s", req.Value, req.Ref)}
}
