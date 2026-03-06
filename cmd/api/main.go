package main

import (
	"log"
	"time"

	"parkieee/internal/bootstrap"
)

func init() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Fatal("failed to load timezone Asia/Jakarta: ", err)
	}
	time.Local = loc
}

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
