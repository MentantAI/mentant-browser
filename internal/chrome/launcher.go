package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Config holds Chrome launch configuration.
type Config struct {
	CDPPort    int
	ProfileDir string
	Headless   bool
}

// Manager handles the Chrome process lifecycle.
type Manager struct {
	config     Config
	executable string
	cmd        *exec.Cmd
	mu         sync.Mutex
	running    bool
}

// NewManager creates a Manager that will launch Chrome with the given config.
func NewManager(cfg Config) *Manager {
	exe := Detect()
	if exe == "" {
		log.Println("WARNING: No Chromium-based browser detected. Install Chrome, Brave, or Edge.")
	}
	return &Manager{
		config:     cfg,
		executable: exe,
	}
}

// Start launches Chrome if not already running.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	if m.executable == "" {
		return fmt.Errorf("no Chromium-based browser found on this system")
	}

	// Ensure profile directory exists
	if err := os.MkdirAll(m.config.ProfileDir, 0755); err != nil {
		return fmt.Errorf("creating profile dir: %w", err)
	}

	// Clear stale lock files from previous crashes
	clearStaleLocks(m.config.ProfileDir)

	flags := BuildFlags(m.config)
	m.cmd = exec.Command(m.executable, flags...)
	m.cmd.Stdout = nil
	m.cmd.Stderr = nil

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("launching Chrome: %w", err)
	}

	log.Printf("Chrome launched (PID %d) on CDP port %d", m.cmd.Process.Pid, m.config.CDPPort)

	// Wait for CDP to become ready
	if err := m.waitForCDP(30 * time.Second); err != nil {
		m.kill()
		return fmt.Errorf("Chrome CDP not ready: %w", err)
	}

	m.running = true

	// Monitor process in background
	go m.monitor()

	return nil
}

// Stop gracefully shuts down Chrome.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running || m.cmd == nil || m.cmd.Process == nil {
		return
	}

	log.Println("Stopping Chrome...")
	m.cmd.Process.Signal(os.Interrupt)

	done := make(chan error, 1)
	go func() { done <- m.cmd.Wait() }()

	select {
	case <-done:
		// Exited cleanly
	case <-time.After(5 * time.Second):
		log.Println("Chrome did not exit, sending SIGKILL")
		m.cmd.Process.Kill()
		<-done
	}

	m.running = false
	log.Println("Chrome stopped")
}

// Running returns whether Chrome is alive.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// CDPEndpoint returns the CDP WebSocket URL for connecting.
func (m *Manager) CDPEndpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d", m.config.CDPPort)
}

// Status returns a map of Chrome state for the /status endpoint.
func (m *Manager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := map[string]any{
		"running":    m.running,
		"executable": m.executable,
		"cdpPort":    m.config.CDPPort,
		"profileDir": m.config.ProfileDir,
	}

	if m.running && m.cmd != nil && m.cmd.Process != nil {
		status["pid"] = m.cmd.Process.Pid
	}

	// Probe CDP for browser version
	if m.running {
		if ver, err := m.browserVersion(); err == nil {
			status["browser"] = ver
		}
	}

	return status
}

func (m *Manager) waitForCDP(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", m.config.CDPPort)

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				log.Println("Chrome CDP ready")
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("CDP not reachable after %v", timeout)
}

func (m *Manager) browserVersion() (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", m.config.CDPPort)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var info struct {
		Browser string `json:"Browser"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.Browser, nil
}

func (m *Manager) monitor() {
	if m.cmd == nil {
		return
	}
	err := m.cmd.Wait()
	m.mu.Lock()
	wasRunning := m.running
	m.running = false
	m.mu.Unlock()

	if wasRunning {
		log.Printf("Chrome exited unexpectedly: %v — restarting...", err)
		time.Sleep(1 * time.Second)
		if err := m.Start(); err != nil {
			log.Printf("Failed to restart Chrome: %v", err)
		}
	}
}

func (m *Manager) kill() {
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
	}
	m.running = false
}

func clearStaleLocks(profileDir string) {
	locks := []string{"SingletonLock", "SingletonCookie", "SingletonSocket"}
	for _, name := range locks {
		path := profileDir + "/" + name
		os.Remove(path) // Ignore errors — file may not exist
	}
}
