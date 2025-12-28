package main

import (
	"context"
	"fmt"
	"github.com/wb-go/wbf/config"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/redis"
	"github.com/wb-go/wbf/zlog"
	"os"
	"os/signal"
	"path/filepath"
	"secondOne/cmd/buildCFG"
	"secondOne/internal/api"
	"secondOne/internal/repo"
	"secondOne/internal/service"
	"syscall"
	"time"
)

func main() {
	zlog.Init()
	log := zlog.Logger
	log.Info().Msg("Hello from zlog")

	cfg := config.New()
	if err := cfg.Load("config.yaml"); err != nil {
		log.Fatal().Msgf("failed to load configuration: %v", err)
	}
	serverCfg := buildCFG.BuildServerConfig(cfg, &log)
	port := serverCfg.Port

	masterDSN, slaveDSNs, poolOptions, err := buildCFG.BuildDBConfig(cfg, &log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build DB config")
	}
	db, err := dbpg.New(masterDSN, slaveDSNs, poolOptions)
	if err != nil {
		log.Fatal().Msgf("failed to connect to DB: %v", err)
	}
	if err := db.Master.Ping(); err != nil {
		log.Fatal().Msgf("DB ping failed: %v", err)
	}
	log.Info().Msg("Database connected successfully")

	redisCfg, err := buildCFG.BuildRedisConfig(cfg, &log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load Redis config")
	}
	rdb := redis.New(redisCfg.Addr, redisCfg.Password, redisCfg.DB)
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Msgf("failed to ping Redis: %v", err)
	}
	log.Info().Msg("Redis connected successfully")

	repository, err := repo.NewRepository(ctx, db, &log)
	if err != nil {
		log.Fatal().Msgf("failed to initialize repository: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get working directory")
	}
	migrationPath := filepath.Join(cwd, "migrations/postgres")
	if err := repository.MigrateUp(migrationPath); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}
	log.Info().Msg("Migrations applied successfully")

	serviceInstance := service.NewService(repository, &log, rdb)
	app := api.NewRouters(&api.Routers{Service: serviceInstance})

	serverErrChan := make(chan error, 1)
	go func() {
		log.Info().Msgf("Starting server on %s", port)
		if err := app.Run(":" + port); err != nil {
			serverErrChan <- fmt.Errorf("failed to start server: %w", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		log.Info().Msgf("Received signal %s. Initiating shutdown...", sig)
	case err := <-serverErrChan:
		log.Error().Msgf("Server error: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if closer, ok := interface{}(app).(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(shutdownCtx); err != nil {
			log.Error().Msgf("Error shutting down server: %v", err)
		}
	}

	log.Info().Msg("Rolling back migrations...")
	if err := repository.MigrateDown(migrationPath); err != nil {
		log.Fatal().Msgf("failed to rollback migrations: %v", err)
	}
	log.Info().Msg("Migrations rolled back successfully")
	log.Info().Msg("Shutdown complete")
}
