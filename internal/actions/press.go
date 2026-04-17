package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// keyMap maps friendly key names to CDP key definitions.
var keyMap = map[string]struct {
	key     string
	code    string
	keyCode int
}{
	"enter":     {"Enter", "Enter", 13},
	"tab":       {"Tab", "Tab", 9},
	"escape":    {"Escape", "Escape", 27},
	"backspace": {"Backspace", "Backspace", 8},
	"delete":    {"Delete", "Delete", 46},
	"arrowup":   {"ArrowUp", "ArrowUp", 38},
	"arrowdown": {"ArrowDown", "ArrowDown", 40},
	"arrowleft": {"ArrowLeft", "ArrowLeft", 37},
	"arrowright":{"ArrowRight", "ArrowRight", 39},
	"home":      {"Home", "Home", 36},
	"end":       {"End", "End", 35},
	"pageup":    {"PageUp", "PageUp", 33},
	"pagedown":  {"PageDown", "PageDown", 34},
	"space":     {" ", "Space", 32},
}

// Press dispatches a keyboard key press event.
func Press(ctx context.Context, req Request) Response {
	if req.Key == "" {
		return Response{OK: false, Error: "key is required for press"}
	}

	keyName := strings.ToLower(req.Key)
	def, ok := keyMap[keyName]
	if !ok {
		// For single characters, type them directly
		if len(req.Key) == 1 {
			err := chromedp.Run(ctx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					if err := input.DispatchKeyEvent(input.KeyDown).
						WithKey(req.Key).Do(ctx); err != nil {
						return err
					}
					if err := input.DispatchKeyEvent(input.KeyChar).
						WithText(req.Key).Do(ctx); err != nil {
						return err
					}
					return input.DispatchKeyEvent(input.KeyUp).
						WithKey(req.Key).Do(ctx)
				}),
			)
			if err != nil {
				return Response{OK: false, Error: fmt.Sprintf("pressing key: %v", err)}
			}
			return Response{OK: true, Message: fmt.Sprintf("pressed %s", req.Key)}
		}
		return Response{OK: false, Error: fmt.Sprintf("unknown key: %q", req.Key)}
	}

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := input.DispatchKeyEvent(input.KeyDown).
				WithKey(def.key).
				WithCode(def.code).
				WithWindowsVirtualKeyCode(int64(def.keyCode)).
				Do(ctx); err != nil {
				return err
			}
			return input.DispatchKeyEvent(input.KeyUp).
				WithKey(def.key).
				WithCode(def.code).
				WithWindowsVirtualKeyCode(int64(def.keyCode)).
				Do(ctx)
		}),
	)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("pressing key: %v", err)}
	}

	return Response{OK: true, Message: fmt.Sprintf("pressed %s", req.Key)}
}
