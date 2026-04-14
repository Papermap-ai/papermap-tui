package main

import (
	"fmt"
	"os"

	"github.com/papermap/papermap-tui/internal/app"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "logout":
			message, err := app.Logout()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Papermap failed: %v\n", err)
				os.Exit(1)
			}

			if message != "" {
				fmt.Fprintln(os.Stdout, message)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "Papermap failed: unknown command %q\n", os.Args[1])
			os.Exit(1)
		}
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Papermap failed: %v\n", err)
		os.Exit(1)
	}
}
