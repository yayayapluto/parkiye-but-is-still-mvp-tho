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
		truncate = flag.Bool("truncate", false, "Truncate all tables before seeding (requires -seed or -seed-only)")
	)
	flag.Parse()

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
		if *truncate {
			fmt.Println("→ Truncating all tables...")
			if err := database.Truncate(db.DB); err != nil {
				log.Fatalf("truncate: %v", err)
			}
			fmt.Println("✓ Truncate complete")
		}

		fmt.Println("→ Running Seeder...")
		if err := database.Seed(db.DB); err != nil {
			log.Fatalf("seed: %v", err)
		}
		fmt.Println("✓ Seeder complete")
	}

	fmt.Println("Done.")
	os.Exit(0)
}
