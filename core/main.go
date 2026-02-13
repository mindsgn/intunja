package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mindsgn-studio/intunja/core/cmd"
)

const version = "2.0.0"

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")

	flag.Parse()

	if *showVersion {
		fmt.Printf("Intunja BitTorrent CLI v%s\n", version)
		os.Exit(0)
	}

	if err := cmd.Run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
