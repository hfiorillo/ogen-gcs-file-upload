package main

import (
	"context"
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
	"github.com/pkg/errors"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/gcs"
	"gitlab.com/totalprocessing/file-upload/internal/handlers"
	"gitlab.com/totalprocessing/file-upload/internal/logs"
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
}

func main() {
	logger := logs.NewPrettyLogger()
	if err := run(logger); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	err := godotenv.Load()
	if err != nil {
		logger.Info("No .env file loaded")
	}

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gcsClient, err := storage.NewClient(ctx,
		option.WithQuotaProject(cfg.GcsProject))
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer gcsClient.Close()

	sec := handlers.NewSecurityHandler(logger,
		cfg.AuthPassword,
		cfg.AuthPassword)

	h := handlers.NewUploadHandler(logger, gcs.GcsClient{
		Logger:    logger,
		GcsClient: gcsClient,
		GcsConfig: gcs.GcsConfig{
			GcsProject:    cfg.GcsProject,
			GcsLocation:   cfg.GcsLocation,
			GcsBucketName: cfg.GcsBucketName},
	})

	fileUploadServer, err := fileupload.NewServer(h, sec,
		fileupload.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
			logger.ErrorContext(ctx, "server error", "error", err)
			ogenerrors.DefaultErrorHandler(ctx, w, r, err)
		}),
	)
	if err != nil {
		logger.Error("failed to create server", "error", err)
		return fmt.Errorf("failed to create server: %v", err)
	}

	// ------- SERVER START
	server := &http.Server{
		Addr:         ":8080",
		Handler:      fileUploadServer,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("application", "available", fmt.Sprintf("localhost%s", server.Addr))
		serverErrors <- server.ListenAndServe()
	}()

	// ------- SHUTDOWN
	select {
	case err := <-serverErrors:
		return errors.Wrap(err, "server error")
	case sig := <-shutdown:
		logger.Info("shutdown", "status", "shutdown started", "signal", sig)
		defer logger.Info("shutdown", "status", "shutdown complete", "signal", sig)

		// ctx, cancel := context.WithTimeout(context.Background(), 2)
		// defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			server.Close()
			return errors.Wrap(err, "could not stop server gracefully")
		}
	}

	return nil
}
