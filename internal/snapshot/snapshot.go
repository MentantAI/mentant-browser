package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

// interactiveRoles are the ARIA roles that get assigned refs for agent interaction.
var interactiveRoles = map[string]bool{
	"button":        true,
	"link":          true,
	"textbox":       true,
	"checkbox":      true,
	"radio":         true,
	"combobox":      true,
	"menuitem":      true,
	"tab":           true,
	"switch":        true,
	"slider":        true,
	"searchbox":     true,
	"spinbutton":    true,
	"option":        true,
	"menuitemradio": true,
	"listbox":       true,
	"treeitem":      true,
}

// RefEntry maps a ref like "e1" to its backing node.
type RefEntry struct {
	BackendNodeID cdp.BackendNodeID `json:"backendNodeId"`
	Role          string            `json:"role"`
	Name          string            `json:"name"`
}

// Result holds a formatted snapshot and its ref mappings.
type Result struct {
	URL      string              `json:"url"`
	Title    string              `json:"title"`
	Text     string              `json:"snapshot"`
	Refs     map[string]RefEntry `json:"refs"`
	TabID    string              `json:"tabId,omitempty"`
	RefCount int                 `json:"refCount"`
}

// refCache stores ref mappings per target (tab). Cleared on navigation.
var (
	refCache   = make(map[string]map[string]RefEntry)
	refCacheMu sync.Mutex
)

// ClearRefs removes cached refs for a target (call on navigation).
func ClearRefs(targetID string) {
	refCacheMu.Lock()
	delete(refCache, targetID)
	refCacheMu.Unlock()
}

// GetCachedRefs returns cached refs for a target, if any.
func GetCachedRefs(targetID string) map[string]RefEntry {
	refCacheMu.Lock()
	defer refCacheMu.Unlock()
	return refCache[targetID]
}

// CacheRefs stores refs for a target.
func CacheRefs(targetID string, refs map[string]RefEntry) {
	refCacheMu.Lock()
	refCache[targetID] = refs
	refCacheMu.Unlock()
}

// Take fetches the accessibility tree from the browser and formats it with refs.
// filter: "interactive" (only actionable elements) or "all" (full tree).
func Take(ctx context.Context, filter string) (*Result, error) {
	// Get current page info
	var url, title string
	if err := chromedp.Run(ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
	); err != nil {
		return nil, fmt.Errorf("getting page info: %w", err)
	}

	// Fetch full accessibility tree
	nodes, err := fetchAccessibilityTree(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching accessibility tree: %w", err)
	}

	// Build refs and formatted text
	interactiveOnly := filter != "all"
	refs, text := formatTree(nodes, interactiveOnly, url, title)

	return &Result{
		URL:      url,
		Title:    title,
		Text:     text,
		Refs:     refs,
		RefCount: len(refs),
	}, nil
}

func fetchAccessibilityTree(ctx context.Context) ([]*accessibility.Node, error) {
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
	return nodes, err
}

// axNode is a simplified node for tree building.
type axNode struct {
	id            string
	role          string
	name          string
	children      []*axNode
	backendNodeID cdp.BackendNodeID
}

func formatTree(nodes []*accessibility.Node, interactiveOnly bool, url, title string) (map[string]RefEntry, string) {
	refs := make(map[string]RefEntry)
	refCounter := 0

	// Build a map of node ID -> node for tree construction
	nodeMap := make(map[string]*axNode)
	var rootIDs []string

	for _, n := range nodes {
		role := valueString(n.Role)
		name := valueString(n.Name)

		an := &axNode{
			id:            string(n.NodeID),
			role:          role,
			name:          name,
			backendNodeID: n.BackendDOMNodeID,
		}
		nodeMap[an.id] = an

		if n.ParentID == "" {
			rootIDs = append(rootIDs, an.id)
		}
	}

	// Wire up children
	for _, n := range nodes {
		if n.ParentID != "" {
			parent := nodeMap[string(n.ParentID)]
			child := nodeMap[string(n.NodeID)]
			if parent != nil && child != nil {
				parent.children = append(parent.children, child)
			}
		}
	}

	// Format the tree
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("page %q [url=%s]\n", title, url))

	for _, rootID := range rootIDs {
		if root := nodeMap[rootID]; root != nil {
			formatNode(&sb, root, 1, interactiveOnly, refs, &refCounter)
		}
	}

	return refs, sb.String()
}

func formatNode(sb *strings.Builder, node *axNode, depth int, interactiveOnly bool, refs map[string]RefEntry, counter *int) {
	if node == nil || node.role == "none" || node.role == "generic" || node.role == "InlineTextBox" {
		for _, child := range node.children {
			formatNode(sb, child, depth, interactiveOnly, refs, counter)
		}
		return
	}

	isInteractive := interactiveRoles[node.role]

	if interactiveOnly && !isInteractive && !hasInteractiveDescendant(node) {
		return
	}

	indent := strings.Repeat("  ", depth)
	name := node.name
	if len(name) > 100 {
		name = name[:100] + "..."
	}

	if isInteractive && node.backendNodeID != 0 {
		*counter++
		ref := fmt.Sprintf("e%d", *counter)
		refs[ref] = RefEntry{
			BackendNodeID: node.backendNodeID,
			Role:          node.role,
			Name:          node.name,
		}
		if name != "" {
			fmt.Fprintf(sb, "%s%s %q [ref=%s]\n", indent, node.role, name, ref)
		} else {
			fmt.Fprintf(sb, "%s%s [ref=%s]\n", indent, node.role, ref)
		}
	} else if !interactiveOnly && name != "" {
		fmt.Fprintf(sb, "%s%s %q\n", indent, node.role, name)
	}

	for _, child := range node.children {
		formatNode(sb, child, depth+1, interactiveOnly, refs, counter)
	}
}

func hasInteractiveDescendant(node *axNode) bool {
	for _, child := range node.children {
		if interactiveRoles[child.role] {
			return true
		}
		if hasInteractiveDescendant(child) {
			return true
		}
	}
	return false
}

func valueString(v *accessibility.Value) string {
	if v == nil || v.Value == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(v.Value, &s); err == nil {
		return s
	}
	return string(v.Value)
}
