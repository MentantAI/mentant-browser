package text

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"
)

// Result holds extracted text.
type Result struct {
	Text string `json:"text"`
	Mode string `json:"mode"`
	URL  string `json:"url"`
}

// Extract gets readable text from the current page.
// mode: "readability" attempts article extraction; "raw" returns all text.
func Extract(ctx context.Context, mode string) (*Result, error) {
	var url string
	if err := chromedp.Run(ctx, chromedp.Location(&url)); err != nil {
		return nil, err
	}

	var text string
	var err error

	if mode == "readability" {
		text, err = extractReadability(ctx)
		if err != nil || text == "" {
			// Fallback to raw
			text, err = extractRaw(ctx)
		}
	} else {
		text, err = extractRaw(ctx)
	}

	if err != nil {
		return nil, err
	}

	// Truncate very long pages
	if len(text) > 50000 {
		text = text[:50000] + "\n... (truncated)"
	}

	return &Result{
		Text: text,
		Mode: mode,
		URL:  url,
	}, nil
}

func extractReadability(ctx context.Context) (string, error) {
	// Use a simplified readability extraction via JavaScript.
	// This removes nav, header, footer, sidebar, ads and extracts the main content.
	var text string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// Try to find the main content area
				const selectors = ['main', 'article', '[role="main"]', '#content', '#main-content', '.content', '.post-content'];
				for (const sel of selectors) {
					const el = document.querySelector(sel);
					if (el && el.innerText.trim().length > 100) {
						return el.innerText.trim();
					}
				}
				// Fallback: clone body, remove known noise, return text
				const clone = document.body.cloneNode(true);
				const noise = clone.querySelectorAll('nav, header, footer, aside, script, style, noscript, [role="navigation"], [role="banner"], [role="contentinfo"], .sidebar, .nav, .footer, .header, .ad, .ads, .advertisement');
				noise.forEach(el => el.remove());
				return clone.innerText.trim();
			})()
		`, &text),
	)
	return text, err
}

func extractRaw(ctx context.Context) (string, error) {
	var text string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.body.innerText`, &text),
	)
	if err != nil {
		return "", fmt.Errorf("extracting text: %w", err)
	}
	return text, nil
}
