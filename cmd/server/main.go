package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/distributed-file-storage/service/src/api"
	"github.com/distributed-file-storage/service/src/api/handlers"
	"github.com/distributed-file-storage/service/src/config"
	"github.com/distributed-file-storage/service/src/core"
	"github.com/distributed-file-storage/service/src/infrastructure/auth"
	"github.com/distributed-file-storage/service/src/infrastructure/database"
	"github.com/distributed-file-storage/service/src/infrastructure/metrics"
	"github.com/distributed-file-storage/service/src/infrastructure/storage"
)

const version = "1.0.0"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(initLogger(cfg))

	slog.Info("starting distributed file storage service", "version", version)

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		slog.Warn("database connection failed, running in limited mode", "error", err)
		db = nil
	}

	if db != nil {
		if err := database.RunMigrations(db, cfg.Database.MigrationsDir); err != nil {
			slog.Error("failed to run migrations", "error", err)
		}
	}

	diskStore, err := storage.NewDiskStore(cfg.Storage.DataDir, cfg.Storage.TempDir)
	if err != nil {
		slog.Error("failed to initialize disk store", "error", err)
		os.Exit(1)
	}

	chunker := storage.NewChunker(cfg.Storage.ChunkSize)
	codec := storage.NewReedSolomonCodec(cfg.Storage.DataShards, cfg.Storage.ParityShards)
	replicator := storage.NewReplicator(cfg.Storage.ReplicationFactor, []*storage.DiskStore{diskStore})

	mm := metrics.NewMetricsManager()

	mm.SetReplicationFactor(cfg.Storage.ReplicationFactor)
	ratio := float64(cfg.Storage.ParityShards) / float64(cfg.Storage.DataShards+cfg.Storage.ParityShards)
	mm.SetErasureCodingRatio(ratio)

	metricsCollector := api.NewSystemMetricsCollector(mm)
	metricsCollector.Start(30 * time.Second)

	userRepo := database.NewUserRepository(db)
	bucketRepo := database.NewBucketRepository(db)
	fileRepo := database.NewFileRepository(db)
	auditRepo := database.NewAuditLogRepository(db)
	_ = database.NewPolicyRepository(db)

	jwtManager := auth.NewJWTManager(
		cfg.Auth.JWTSecret,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
	)
	rbac := auth.NewRBACEngine()

	fileService := core.NewFileService(cfg, fileRepo, bucketRepo, diskStore, chunker, codec, replicator, mm, auditRepo)
	bucketService := core.NewBucketService(bucketRepo, fileRepo, auditRepo, rbac, mm)

	bucketHandler := handlers.NewBucketHandler(bucketService)
	fileHandler := handlers.NewFileHandler(fileService)
	authHandler := handlers.NewAuthHandler(userRepo, jwtManager, cfg)
	healthHandler := handlers.NewHealthHandler(version)

	router := api.NewRouter(cfg, jwtManager, rbac, mm, bucketHandler, fileHandler, authHandler, healthHandler)

	srv := &http.Server{
		Addr:         cfg.Server.Host + ":" + intToString(cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		slog.Info("listening for connections", "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("received shutdown signal", "signal", sig.String())

	cleanupCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if db != nil {
		db.Close()
	}

	if err := srv.Shutdown(cleanupCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server stopped gracefully")
}

func initLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	if result == "" {
		result = "0"
	}
	return result
}
