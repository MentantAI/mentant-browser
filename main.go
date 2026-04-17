package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/MentantAI/mentant-browser/internal/chrome"
	"github.com/MentantAI/mentant-browser/internal/server"
)

var version = "dev"

func main() {
	port := flag.Int("port", 9876, "HTTP server port")
	cdpPort := flag.Int("cdp-port", 9877, "Chrome CDP port")
	profileDir := flag.String("profile", defaultProfileDir(), "Chrome user data directory")
	headless := flag.Bool("headless", false, "Run Chrome in headless mode")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("mentant-browser %s\n", version)
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("mentant-browser %s starting on :%d (CDP :%d)", version, *port, *cdpPort)

	mgr := chrome.NewManager(chrome.Config{
		CDPPort:    *cdpPort,
		ProfileDir: *profileDir,
		Headless:   *headless,
	})

	srv := server.New(mgr, *port)

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		srv.Stop()
		mgr.Stop()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func defaultProfileDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/mentant-browser/profiles/default"
	}
	return home + "/.mentant/browser/profiles/default"
}
