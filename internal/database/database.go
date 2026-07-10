package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/warm3snow/llm-gateway/internal/metrics"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds database configuration
type Config struct {
	Driver          string        // "sqlite" or "postgres"
	DSN             string        // Data source name
	LogLevel        string        // "silent", "error", "warn", "info"
	MaxOpenConns    int           // maximum open connections; 0 uses database/sql default
	MaxIdleConns    int           // maximum idle connections; 0 uses database/sql default
	ConnMaxLifetime time.Duration // maximum connection reuse lifetime; 0 disables expiry
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
	sqlDB, err := configurePool(db, cfg)
	if err != nil {
		return err
	}
	metrics.ObserveDatabasePool(cfg.Driver, sqlDB)

	DB = db
	log.Printf("[DATABASE] Connected to %s database", cfg.Driver)
	return nil
}

func configurePool(db *gorm.DB, cfg *Config) (*sql.DB, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to access sql database: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	return sqlDB, nil
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

// TenantScope returns a GORM scope that filters queries to a single tenant.
// A tenantID of 0 disables filtering (used by super_admin / cross-tenant
// queries). Apply it via db.Scopes(database.TenantScope(tenantID)).
func TenantScope(tenantID uint) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if tenantID == 0 {
			return db
		}
		return db.Where("tenant_id = ?", tenantID)
	}
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		metrics.ObserveDatabasePool("", nil)
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	metrics.ObserveDatabasePool("", nil)

	return sqlDB.Close()
}
