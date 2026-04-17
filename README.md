# mentant-browser

A standalone HTTP server that manages a Chromium browser instance via the Chrome DevTools Protocol (CDP). Built for [Mentant](https://mentant.ai) AI agents, but usable by any tool that speaks HTTP + JSON.

## What it does

- **Manages Chrome lifecycle** — auto-detects Chrome/Brave/Edge, launches with a persistent profile, restarts on crash
- **Accessibility-tree snapshots** — returns the page as a structured tree with numbered refs (`e1`, `e2`, ...) for AI-friendly interaction
- **Ref-based actions** — click, type, fill, press, scroll, select by ref instead of fragile CSS selectors
- **Persistent sessions** — cookies and login state survive between requests (stored at `~/.mentant/browser/profiles/default/`)
- **Text extraction** — readability-mode content extraction (~800 tokens/page vs 10k+ for screenshots)
- **Screenshots** — viewport or full-page PNG capture
- **Tab management** — list, open, close browser tabs
- **Zero runtime dependencies** — single static Go binary, no Node.js/npm required

## Install

**Via Go:**

```sh
go install github.com/MentantAI/mentant-browser@latest
```

**Via Mentant:**

```sh
mentant install  # installs automatically
```

**From releases:**

Download the binary for your platform from [GitHub Releases](https://github.com/MentantAI/mentant-browser/releases).

## Usage

```sh
# Start the server (launches Chrome automatically on first request)
mentant-browser

# With options
mentant-browser --port 9876 --cdp-port 9877 --profile ~/.mentant/browser/profiles/default
mentant-browser --headless
mentant-browser --version
```

## API

The server listens on `http://127.0.0.1:9876` (loopback only).

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/status` | GET | Server + Chrome health |
| `/start` | POST | Launch Chrome |
| `/stop` | POST | Stop Chrome |
| `/tabs` | GET | List open tabs |
| `/tabs` | POST | Open a new tab (`{"url": "..."}`) |
| `/tabs/:id` | DELETE | Close a tab |
| `/navigate` | POST | Navigate to URL (`{"url": "..."}`) |
| `/snapshot` | GET | Accessibility tree with refs (`?filter=interactive\|all`) |
| `/act` | POST | Perform an action (see below) |
| `/screenshot` | POST | Capture PNG (`{"fullPage": true}`) |
| `/text` | GET | Extract page text (`?mode=readability\|raw`) |

### Snapshot

```sh
curl http://localhost:9876/snapshot
```

Returns:

```json
{
  "url": "https://example.com/",
  "title": "Example Domain",
  "snapshot": "page \"Example Domain\" [url=https://example.com/]\n  link \"Learn more\" [ref=e1]\n",
  "refs": {
    "e1": {"backendNodeId": 15, "role": "link", "name": "Learn more"}
  },
  "refCount": 1
}
```

### Actions

All actions go through `POST /act` with a `kind` field:

```sh
# Click
curl -X POST localhost:9876/act -d '{"kind":"click","ref":"e1"}'

# Type (character-by-character, SPA-friendly)
curl -X POST localhost:9876/act -d '{"kind":"type","ref":"e3","text":"hello"}'

# Fill (set value directly)
curl -X POST localhost:9876/act -d '{"kind":"fill","ref":"e3","value":"hello"}'

# Press a key
curl -X POST localhost:9876/act -d '{"kind":"press","key":"Enter"}'

# Scroll into view
curl -X POST localhost:9876/act -d '{"kind":"scroll","ref":"e5"}'

# Select dropdown option
curl -X POST localhost:9876/act -d '{"kind":"select","ref":"e2","value":"Option A"}'

# Wait for element
curl -X POST localhost:9876/act -d '{"kind":"wait","text":"Success","timeout":10}'
```

### Workflow

The typical agent workflow is:

1. `POST /navigate` — go to a page
2. `GET /snapshot` — see what's on the page (with refs)
3. `POST /act` — interact using refs from the snapshot
4. Repeat 2-3 as needed

## Architecture

```
mentant-browser (Go, net/http)
       ↓ CDP over WebSocket
Chrome/Brave/Edge (persistent profile)
```

- **Chrome management**: auto-detect, launch with stealth flags, monitor for crashes, graceful shutdown
- **CDP via chromedp**: accessibility tree, DOM manipulation, input dispatch, screenshots
- **Ref system**: walk the accessibility tree, assign `e1`/`e2`/`e3` to interactive elements (buttons, links, inputs, etc.), cache refs per tab, clear on navigation

## Development

```sh
# Build
make build

# Run
./mentant-browser

# Test
make test

# Cross-compile all platforms
make cross
```

## License

[O'SaaSy License](LICENSE.md) — Copyright © 2026, Mikel Lindsaar
