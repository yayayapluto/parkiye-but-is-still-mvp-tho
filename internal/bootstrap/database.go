package bootstrap

import (
	"parkieee/database"
	"parkieee/pkg/config"
	//"parkieee/pkg/logger"

	"gorm.io/gorm"
)

func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	// Create database connection
	db, err := database.New(&cfg.Database)
	if err != nil {
		return nil, err
	}

	// Run migrations in development only
	if cfg.IsDevelopment() {
		if err := db.RunMigrations(); err != nil {
			return nil, err
		}
	}

	return db.DB, nil
}
