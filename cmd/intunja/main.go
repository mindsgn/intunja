package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mindsgn-studio/intunja/engine"
	"github.com/mindsgn-studio/intunja/tui"
)

func main() {
	// Check command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: bittorrent-client <torrent-file>")
		fmt.Println("\nOptions:")
		fmt.Println("  --daemon    Run in daemon mode (no TUI)")
		os.Exit(1)
	}

	torrentFile := os.Args[1]

	// Parse torrent file
	metaInfo, err := engine.ParseMetaInfo(torrentFile)
	if err != nil {
		fmt.Printf("Error parsing torrent: %v\n", err)
		os.Exit(1)
	}

	// Check for daemon mode
	daemonMode := false
	for _, arg := range os.Args {
		if arg == "--daemon" {
			daemonMode = true
			break
		}
	}

	if daemonMode {
		runDaemon(metaInfo)
	} else {
		runTUI(metaInfo)
	}
}

// runDaemon runs the client in headless daemon mode
func runDaemon(metaInfo *engine.MetaInfo) {
	fmt.Printf("Starting download: %s\n", metaInfo.Info.Name)
	fmt.Printf("Total size: %s\n", formatBytes(metaInfo.TotalLength()))
	fmt.Printf("Pieces: %d\n", metaInfo.NumPieces())

	// Create download manager
	manager := engine.NewDownloadManager(metaInfo, "./downloads")

	// Start download
	if err := manager.Start(); err != nil {
		fmt.Printf("Error starting download: %v\n", err)
		os.Exit(1)
	}

	// Monitor progress
	fmt.Println("\nDownloading...")
	for {
		progress := manager.GetProgress()
		downloaded, uploaded, peers := manager.GetStats()

		fmt.Printf("\rProgress: %.1f%% | Downloaded: %s | Uploaded: %s | Peers: %d",
			progress*100,
			formatBytes(downloaded),
			formatBytes(uploaded),
			peers,
		)

		if progress >= 1.0 {
			fmt.Println("\n\nDownload complete!")
			break
		}
	}

	manager.Wait()
}

// runTUI runs the client with the Bubble Tea interface
func runTUI(metaInfo *engine.MetaInfo) {
	// Create TUI model
	model := tui.NewModel()

	// Create download manager
	manager := engine.NewDownloadManager(metaInfo, "./downloads")

	// Start download in background
	go func() {
		if err := manager.Start(); err != nil {
			fmt.Printf("Error starting download: %v\n", err)
			os.Exit(1)
		}
	}()

	// Run TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
