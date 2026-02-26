package main

import (
	"log"

	"parkieee/internal/bootstrap"
)

func main() {
	// Create application
	app, err := bootstrap.NewApp()
	if err != nil {
		log.Fatal("Failed to initialize application: ", err)
	}

	// Run application
	if err := app.Run(); err != nil {
		app.Logger.Error(nil, "application error", "error", err)
	}
}
