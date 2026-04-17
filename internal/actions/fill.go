package actions

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Fill sets an input element's value directly (faster than type, but may not
// trigger React/Vue handlers). For SPAs, prefer Type.
func Fill(ctx context.Context, req Request, resolver RefResolver) Response {
	if req.Ref == "" {
		return Response{OK: false, Error: "ref is required for fill"}
	}
	value := req.Value
	if value == "" {
		value = req.Text
	}

	backendNodeID, err := resolver(req.Ref)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("resolving ref %q: %v", req.Ref, err)}
	}

	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Resolve node to a remote object
			obj, err := dom.ResolveNode().WithBackendNodeID(backendNodeID).Do(ctx)
			if err != nil {
				return fmt.Errorf("resolving node: %w", err)
			}

			// Focus the element
			if err := dom.Focus().WithBackendNodeID(backendNodeID).Do(ctx); err != nil {
				return fmt.Errorf("focusing: %w", err)
			}

			// Set value and dispatch input event
			script := `
				function(value) {
					const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
						window.HTMLInputElement.prototype, 'value'
					)?.set || Object.getOwnPropertyDescriptor(
						window.HTMLTextAreaElement.prototype, 'value'
					)?.set;
					if (nativeInputValueSetter) {
						nativeInputValueSetter.call(this, value);
					} else {
						this.value = value;
					}
					this.dispatchEvent(new Event('input', { bubbles: true }));
					this.dispatchEvent(new Event('change', { bubbles: true }));
				}
			`

			_, _, err = runtime.CallFunctionOn(script).
				WithObjectID(obj.ObjectID).
				WithArguments([]*runtime.CallArgument{{Value: []byte(fmt.Sprintf("%q", value))}}).
				Do(ctx)
			return err
		}),
	)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("filling: %v", err)}
	}

	return Response{OK: true, Message: fmt.Sprintf("filled %s", req.Ref)}
}
