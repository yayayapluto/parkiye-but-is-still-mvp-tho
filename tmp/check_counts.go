package main

import (
	"fmt"
	"log"
	"parkieee/database"
	"parkieee/pkg/config"
)

func main() {
	config.LoadEnv(".env")
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tables := []string{"zones", "gates", "users"}
	for _, t := range tables {
		var count int64
		db.DB.Table(t).Count(&count)
		
		var inactive int64
		if t != "users" {
			db.DB.Table(t).Where("is_active = ?", false).Count(&inactive)
		} else {
			db.DB.Table(t).Where("is_active = ?", false).Count(&inactive)
		}
		
		fmt.Printf("%s: %d total, %d inactive\n", t, count, inactive)
	}
}
