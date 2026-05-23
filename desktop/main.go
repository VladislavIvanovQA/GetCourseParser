package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/getcourse-downloader/getcourse-downloader/internal/config"
	"github.com/getcourse-downloader/getcourse-downloader/internal/ffmpeg"
	"github.com/getcourse-downloader/getcourse-downloader/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := os.MkdirAll(cfg.DownloadsPath(), 0o755); err != nil {
		log.Fatalf("downloads dir: %v", err)
	}

	appDir := config.AppDir()
	if ff, local := ffmpeg.Find(); ff == "" {
		log.Printf("WARNING: install ffmpeg (brew install ffmpeg) or put binary in: %s", appDir)
	} else if local {
		log.Printf("ffmpeg: %s", ff)
	} else {
		log.Printf("ffmpeg (PATH): %s", ff)
	}

	fmt.Println("GetCourse Downloader")
	fmt.Printf("  Folder: %s\n", appDir)
	fmt.Printf("  API:    http://127.0.0.1:%d\n", cfg.Port)
	fmt.Printf("  Token:  %s  (extension uses this once)\n", cfg.Token)
	fmt.Printf("  Saves:  %s\n", cfg.DownloadsPath())
	fmt.Println("Load extension from ../extension in Chrome, then connect.")
	fmt.Println()

	srv := server.New(cfg)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\nBye.")
}
