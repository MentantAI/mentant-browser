package chrome

import "fmt"

// BuildFlags returns Chrome launch arguments for a managed browser instance.
func BuildFlags(cfg Config) []string {
	flags := []string{
		fmt.Sprintf("--remote-debugging-port=%d", cfg.CDPPort),
		fmt.Sprintf("--user-data-dir=%s", cfg.ProfileDir),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-features=Translate,MediaRouter",
		"--hide-crash-restore-bubble",
		"--password-store=basic",
		// Stealth: reduce automation detection
		"--disable-blink-features=AutomationControlled",
	}

	if cfg.Headless {
		flags = append(flags, "--headless=new")
	}

	return flags
}
