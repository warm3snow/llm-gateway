package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds database configuration
type Config struct {
	Driver   string // "sqlite" or "postgres"
	DSN      string // Data source name
	LogLevel string // "silent", "error", "warn", "info"
}

// DB is the global database instance
var DB *gorm.DB

// Connect initializes the database connection
func Connect(cfg *Config) error {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(cfg.DSN)
	case "sqlite":
		// Ensure directory exists
		dir := filepath.Dir(cfg.DSN)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create database directory: %w", err)
			}
		}
		dialector = sqlite.Open(cfg.DSN)
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	logLevel := logger.Warn
	switch cfg.LogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "info":
		logLevel = logger.Info
	}

	gormConfig := &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		),
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	DB = db
	log.Printf("[DATABASE] Connected to %s database", cfg.Driver)
	return nil
}

// Migrate runs auto-migration for all models
func Migrate(models ...interface{}) error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	err := DB.AutoMigrate(models...)
	if err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	log.Println("[DATABASE] Migration completed")
	return nil
}

// GetDB returns the global database instance
func GetDB() *gorm.DB {
	return DB
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
