package main

import (
	"fmt"
	"os"

	"github.com/fezcode/atlas.archive/internal/tui"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-v" || arg == "--version" {
			fmt.Printf("atlas.archive v%s\n", Version)
			return
		}
		if arg == "-h" || arg == "--help" || arg == "help" {
			showHelp()
			return
		}
	}

	if err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("Atlas Archive - A beautiful TUI archiver and extractor.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  atlas.archive              Start the interactive TUI")
	fmt.Println("  atlas.archive -v           Show version")
	fmt.Println("  atlas.archive -h           Show this help")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  - Extract and create archives interactively")
	fmt.Println("  - Pick files with a beautiful file browser")
	fmt.Println("  - Select compression algorithms (zip, tar, tgz, etc.)")
	fmt.Println("  - Cross-platform (Windows, Linux, macOS)")
}
