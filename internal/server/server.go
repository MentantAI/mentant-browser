package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/exec-io/mentant-browser/internal/actions"
	"github.com/exec-io/mentant-browser/internal/chrome"
	"github.com/exec-io/mentant-browser/internal/screenshot"
	"github.com/exec-io/mentant-browser/internal/snapshot"
	"github.com/exec-io/mentant-browser/internal/text"
)

// Server is the HTTP control server for mentant-browser.
type Server struct {
	mgr  *chrome.Manager
	port int
	srv  *http.Server

	// chromedp context — created once Chrome is running, reused across requests
	allocCtx    context.Context
	allocCancel context.CancelFunc
	taskCtx     context.Context
	taskCancel  context.CancelFunc
	ctxMu       sync.Mutex
}

// New creates a server backed by the given Chrome manager.
func New(mgr *chrome.Manager, port int) *Server {
	return &Server{
		mgr:  mgr,
		port: port,
	}
}

// Start begins listening. Blocks until the server is stopped.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:      logMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	log.Printf("Listening on http://127.0.0.1:%d", s.port)
	return s.srv.ListenAndServe()
}

// Stop shuts down the HTTP server.
func (s *Server) Stop() {
	s.ctxMu.Lock()
	if s.taskCancel != nil {
		s.taskCancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
	s.ctxMu.Unlock()

	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.srv.Shutdown(ctx)
	}
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("POST /start", s.handleStart)
	mux.HandleFunc("POST /stop", s.handleStop)
	mux.HandleFunc("GET /tabs", s.handleTabs)
	mux.HandleFunc("POST /tabs", s.handleOpenTab)
	mux.HandleFunc("DELETE /tabs/{id}", s.handleCloseTab)
	mux.HandleFunc("POST /navigate", s.handleNavigate)
	mux.HandleFunc("GET /snapshot", s.handleSnapshot)
	mux.HandleFunc("POST /act", s.handleAct)
	mux.HandleFunc("POST /screenshot", s.handleScreenshot)
	mux.HandleFunc("GET /text", s.handleText)
}

// ensureCDP makes sure Chrome is running and we have a chromedp context.
func (s *Server) ensureCDP() (context.Context, error) {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()

	// Start Chrome if not running
	if !s.mgr.Running() {
		if err := s.mgr.Start(); err != nil {
			return nil, fmt.Errorf("starting Chrome: %w", err)
		}
	}

	// Create chromedp context if we don't have one
	if s.taskCtx == nil {
		cdpURL := s.mgr.CDPEndpoint()
		s.allocCtx, s.allocCancel = chromedp.NewRemoteAllocator(context.Background(), cdpURL)
		s.taskCtx, s.taskCancel = chromedp.NewContext(s.allocCtx)

		// Run a no-op to establish the connection
		if err := chromedp.Run(s.taskCtx); err != nil {
			s.taskCancel()
			s.allocCancel()
			s.taskCtx = nil
			return nil, fmt.Errorf("connecting to Chrome: %w", err)
		}
	}

	return s.taskCtx, nil
}

// resetCDP tears down the chromedp context (e.g. after Chrome restart).
func (s *Server) resetCDP() {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	if s.taskCancel != nil {
		s.taskCancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
	s.taskCtx = nil
}

func (s *Server) refResolver(refs map[string]snapshot.RefEntry) actions.RefResolver {
	return func(ref string) (cdp.BackendNodeID, error) {
		entry, ok := refs[ref]
		if !ok {
			return 0, fmt.Errorf("ref %q not found — run snapshot first", ref)
		}
		return entry.BackendNodeID, nil
	}
}

// --- Handlers ---

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.mgr.Status()
	status["server"] = "ok"
	writeJSON(w, 200, status)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.Start(); err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "Chrome started"})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	s.resetCDP()
	s.mgr.Stop()
	writeJSON(w, 200, map[string]any{"ok": true, "message": "Chrome stopped"})
}

func (s *Server) handleTabs(w http.ResponseWriter, r *http.Request) {
	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	targets, err := chromedp.Targets(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	var tabs []map[string]any
	for _, t := range targets {
		if t.Type == "page" {
			tabs = append(tabs, map[string]any{
				"id":    t.TargetID.String(),
				"url":   t.URL,
				"title": t.Title,
			})
		}
	}

	writeJSON(w, 200, map[string]any{"tabs": tabs})
}

func (s *Server) handleOpenTab(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	newCtx, _ := chromedp.NewContext(ctx)
	if body.URL != "" {
		err = chromedp.Run(newCtx, chromedp.Navigate(body.URL))
	} else {
		err = chromedp.Run(newCtx)
	}
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleCloseTab(w http.ResponseWriter, r *http.Request) {
	tabID := r.PathValue("id")
	if tabID == "" {
		writeJSON(w, 400, map[string]any{"error": "tab id required"})
		return
	}

	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Cancel(ctx)
		}),
	)
	if err != nil && !strings.Contains(err.Error(), "canceled") {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleNavigate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL   string `json:"url"`
		TabID string `json:"tabId"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.URL == "" {
		writeJSON(w, 400, map[string]any{"error": "url is required"})
		return
	}

	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	// Clear refs for this tab since navigation invalidates them
	if body.TabID != "" {
		snapshot.ClearRefs(body.TabID)
	}

	var url, title string
	err = chromedp.Run(ctx,
		chromedp.Navigate(body.URL),
		chromedp.WaitReady("body"),
		chromedp.Location(&url),
		chromedp.Title(&title),
	)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]any{
		"ok":    true,
		"url":   url,
		"title": title,
	})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "interactive"
	}

	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	result, err := snapshot.Take(ctx, filter)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	// Cache refs for subsequent actions
	tabID := r.URL.Query().Get("tabId")
	if tabID == "" {
		tabID = "default"
	}
	snapshot.CacheRefs(tabID, result.Refs)

	writeJSON(w, 200, result)
}

func (s *Server) handleAct(w http.ResponseWriter, r *http.Request) {
	var req actions.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid request body"})
		return
	}

	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	// Get cached refs
	tabID := req.TabID
	if tabID == "" {
		tabID = "default"
	}
	refs := snapshot.GetCachedRefs(tabID)
	if refs == nil {
		refs = make(map[string]snapshot.RefEntry)
	}

	resolver := s.refResolver(refs)
	result := actions.Dispatch(ctx, req, resolver)

	if result.OK {
		writeJSON(w, 200, result)
	} else {
		writeJSON(w, 422, result)
	}
}

func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FullPage bool `json:"fullPage"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	result, err := screenshot.Take(ctx, body.FullPage)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, 200, result)
}

func (s *Server) handleText(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "readability"
	}

	ctx, err := s.ensureCDP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	result, err := text.Extract(ctx, mode)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, 200, result)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
