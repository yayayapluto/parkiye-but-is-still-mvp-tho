package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"parkieee/database"
	"parkieee/pkg/config"
)

func main() {
	var (
		envFile  = flag.String("env", ".env", "Path to .env file")
		runSeed  = flag.Bool("seed", false, "Run seeder after migration")
		seedOnly = flag.Bool("seed-only", false, "Run seeder without migration")
	)
	flag.Parse()

	// Load .env before anything else
	config.LoadEnv(*envFile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatalf("database connection: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("warn: close db: %v", err)
		}
	}()

	if !*seedOnly {
		fmt.Println("→ Running AutoMigrate...")
		if err := db.RunMigrations(); err != nil {
			log.Fatalf("migration: %v", err)
		}
		fmt.Println("✓ Migration complete")
	}

	if *runSeed || *seedOnly {
		fmt.Println("→ Running Seeder...")
		if err := database.Seed(db.DB); err != nil {
			log.Fatalf("seed: %v", err)
		}
		fmt.Println("✓ Seeder complete")
	}

	fmt.Println("Done.")
	os.Exit(0)
}
