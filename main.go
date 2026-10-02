package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"library-cli/db"
	"library-cli/ui"
)

func main() {
	if _, err := db.InitDB("library.db"); err != nil {
		fmt.Fprintf(os.Stderr, "database: %v\n", err)
		os.Exit(1)
	}
	defer db.DB.Close()

	program := tea.NewProgram(ui.New(), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "library: %v\n", err)
		os.Exit(1)
	}
}
