package main

import (
	"fmt"
	"os"

	"github.com/papermap/papermap-tui/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Papermap failed: %v\n", err)
		os.Exit(1)
	}
}
