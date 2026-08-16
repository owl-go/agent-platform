package gormdb

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	DSN                string
	MaxOpenConnections int
	MaxIdleConnections int
	ConnectionMaxIdle  time.Duration
	ConnectionMaxLife  time.Duration
}

type Database struct {
	db *gorm.DB
}

func Open(ctx context.Context, config Config) (*Database, error) {
	if config.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}
	if config.MaxOpenConnections <= 0 || config.MaxIdleConnections < 0 || config.MaxIdleConnections > config.MaxOpenConnections {
		return nil, fmt.Errorf("database connection limits are invalid")
	}
	if config.ConnectionMaxIdle <= 0 || config.ConnectionMaxLife <= 0 {
		return nil, fmt.Errorf("database connection lifetimes must be positive")
	}
	db, err := gorm.Open(postgres.Open(config.DSN), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL with GORM: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access GORM connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(config.MaxOpenConnections)
	sqlDB.SetMaxIdleConns(config.MaxIdleConnections)
	sqlDB.SetConnMaxIdleTime(config.ConnectionMaxIdle)
	sqlDB.SetConnMaxLifetime(config.ConnectionMaxLife)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	if err := Migrate(ctx, db); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &Database{db: db}, nil
}

func (database *Database) ORM() *gorm.DB {
	return database.db
}

func (database *Database) PingContext(ctx context.Context) error {
	sqlDB, err := database.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (database *Database) Close() error {
	sqlDB, err := database.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
