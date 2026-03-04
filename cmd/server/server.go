package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/ogen-go/ogen/ogenerrors"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/gcs"
	"gitlab.com/totalprocessing/file-upload/internal/handlers"
	"gitlab.com/totalprocessing/file-upload/internal/logs"
	"gitlab.com/totalprocessing/file-upload/internal/observability"

	"google.golang.org/api/option"
)

type Config struct {
	Port            string        `env:"PORT" envDefault:"8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"5s"`

	GcsProject    string `env:"GCS_PROJECT,required,notEmpty"`
	GcsBucketName string `env:"GCS_BUCKET_NAME,required,notEmpty"`
	GcsLocation   string `env:"GCS_LOCATION" envDefault:"global"`

	AuthUsername string `env:"AUTH_USERNAME,required,notEmpty"`
	AuthPassword string `env:"AUTH_PASSWORD,required,notEmpty"`

	FileUploadLimit int `env:"FILE_UPLOAD_LIMIT" envDefault:"10"`

	Environment       string  `env:"ENVIRONMENT" envDefault:"development"`
	TracingEnabled    bool    `env:"TRACING_ENABLED" envDefault:"false"`
	TracingEndpoint   string  `env:"TRACING_ENDPOINT"`
	TracingSampleRate float64 `env:"TRACING_SAMPLE_RATE" envDefault:"1.0"`
}

func main() {
	_ = godotenv.Load()

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		logs.NewLogger("local").Error("server", "error", fmt.Errorf("parsing config: %w", err))
		os.Exit(1)
	}

	logger := logs.NewLogger(cfg.Environment)
	if err := run(cfg, logger); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}

func run(cfg Config, logger *slog.Logger) (err error) {
	logger.Info(
		"configuration loaded",
		"port", cfg.Port,
		"gcs_project", cfg.GcsProject,
		"gcs_bucket", cfg.GcsBucketName,
		"file_upload_limit_mb", cfg.FileUploadLimit,
		"environment", cfg.Environment,
		"tracing_enabled", cfg.TracingEnabled,
		"tracing_endpoint", cfg.TracingEndpoint,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gcsClient, err := storage.NewClient(ctx,
		option.WithQuotaProject(cfg.GcsProject),
	)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer gcsClient.Close()

	sec := handlers.NewSecurityHandler(logger,
		cfg.AuthUsername,
		cfg.AuthPassword,
	)

	maxUploadSizeBytes := int64(cfg.FileUploadLimit) * 1024 * 1024

	h := handlers.NewUploadHandler(logger, gcs.GcsClient{
		Logger:    logger,
		GcsClient: gcsClient,
		GcsConfig: gcs.GcsConfig{
			GcsProject:         cfg.GcsProject,
			GcsLocation:        cfg.GcsLocation,
			GcsBucketName:      cfg.GcsBucketName,
			MaxUploadSizeBytes: maxUploadSizeBytes,
		},
	})

	telemetry, err := observability.New(observability.Config{
		ServiceName:       "http-file-upload",
		ServiceVersion:    "v1",
		Environment:       cfg.Environment,
		Logger:            logger,
		TracingEnabled:    cfg.TracingEnabled,
		TracingEndpoint:   cfg.TracingEndpoint,
		TracingSampleRate: cfg.TracingSampleRate,
	})
	if err != nil {
		return fmt.Errorf("failed to setup observability: %w", err)
	}

	receivedShutdownSignal := false

	defer func() {
		if receivedShutdownSignal {
			logger.Info("shutdown", "status", "shutdown complete")
		}
	}()

	// handle shutdown properly so nothing leaks
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		err = errors.Join(err, telemetry.Shutdown(shutdownCtx))
	}()

	fileUploadServer, err := fileupload.NewServer(h, sec,
		fileupload.WithMiddleware(),
		fileupload.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
			logger.ErrorContext(ctx, "server error", "error", err)
			ogenerrors.DefaultErrorHandler(ctx, w, r, err)
		}),
		fileupload.WithMeterProvider(telemetry.MeterProvider),
		fileupload.WithTracerProvider(telemetry.TracerProvider),
	)

	if err != nil {
		logger.Error("failed to create server", "error", err)
		return fmt.Errorf("failed to create server: %v", err)
	}

	// ------- SERVER START
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      fileUploadServer,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("application", "available", fmt.Sprintf("localhost%s", server.Addr))
		serverErrors <- server.ListenAndServe()
	}()

	// ------- SHUTDOWN
	select {
	case serverErr := <-serverErrors:
		if errors.Is(serverErr, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server error: %w", serverErr)
	case sig := <-shutdown:
		receivedShutdownSignal = true
		logger.Info("shutdown", "status", "shutdown started", "signal", sig)

		// Prevent infinite waiting
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}
