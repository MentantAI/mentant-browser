package chrome

import (
	"os/exec"
	"runtime"
)

// Detect finds the first available Chromium-based browser on the system.
// Search order: Chrome, Brave, Edge, Chromium.
func Detect() string {
	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		}
	case "linux":
		candidates = []string{
			"google-chrome",
			"google-chrome-stable",
			"brave-browser",
			"microsoft-edge",
			"chromium",
			"chromium-browser",
		}
	default:
		candidates = []string{
			"chrome",
			"chromium",
		}
	}

	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return path
		}
		// On macOS the full path won't be on PATH but is valid directly
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := exec.LookPath(path)
	if err == nil {
		return true
	}
	// Try stat for absolute paths
	_, err = exec.Command("test", "-x", path).Output()
	return err == nil
}
