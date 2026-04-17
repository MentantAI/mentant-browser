package actions

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
)

// Request represents a unified action request from the API.
type Request struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref,omitempty"`
	Value   string `json:"value,omitempty"`
	Key     string `json:"key,omitempty"`
	Text    string `json:"text,omitempty"`
	TabID   string `json:"tabId,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // seconds, for wait
}

// Response is returned from action execution.
type Response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RefResolver looks up a ref (e.g. "e1") and returns the backend DOM node ID.
type RefResolver func(ref string) (cdp.BackendNodeID, error)

// Dispatch executes an action based on its kind.
func Dispatch(ctx context.Context, req Request, resolver RefResolver) Response {
	switch req.Kind {
	case "click":
		return Click(ctx, req, resolver)
	case "type":
		return Type(ctx, req, resolver)
	case "fill":
		return Fill(ctx, req, resolver)
	case "press":
		return Press(ctx, req)
	case "scroll":
		return Scroll(ctx, req, resolver)
	case "select":
		return Select(ctx, req, resolver)
	case "wait":
		return Wait(ctx, req, resolver)
	default:
		return Response{OK: false, Error: fmt.Sprintf("unknown action kind: %q", req.Kind)}
	}
}
