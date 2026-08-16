package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agent-platform/backend/internal/conf"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/platformconfig"
)

func main() {
	configPath := flag.String("config", platformconfig.DefaultPath, "path to the YAML platform configuration")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *configPath); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, configPath string) error {
	config, err := conf.Load(configPath)
	if err != nil {
		return err
	}
	if err := config.ValidateAPI(); err != nil {
		return err
	}
	startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db, err := gormdb.Open(startupCtx, databaseConfig(config.Database))
	if err != nil {
		return err
	}
	defer db.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		"service", "agent-platform-api",
		"version", "dev",
		"instance", instanceID(),
	)
	app, err := initializeAPI(ctx, config, db, logger)
	if err != nil {
		return err
	}
	logger.Info("api starting", "address", config.API.Address)
	return app.Run()
}

func instanceID() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "unknown"
	}
	return name
}

func databaseConfig(config platformconfig.DatabaseConfig) gormdb.Config {
	return gormdb.Config{
		DSN: config.DSN, MaxOpenConnections: config.MaxOpenConnections,
		MaxIdleConnections: config.MaxIdleConnections, ConnectionMaxIdle: config.ConnectionMaxIdle.Value(),
		ConnectionMaxLife: config.ConnectionMaxLife.Value(),
	}
}
