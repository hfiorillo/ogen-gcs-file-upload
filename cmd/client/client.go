package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/ogen-go/ogen/http"
	"gitlab.com/totalprocessing/file-upload/internal/auth"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/logs"
	"gitlab.com/totalprocessing/file-upload/internal/tracing"

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

	ClientUrl string `env:"CLIENT_URL" envDefault:"localhost:8080"`
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

	// ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	// defer stop()

	gcsClient, err := storage.NewClient(ctx,
		option.WithQuotaProject(cfg.GcsProject))
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer gcsClient.Close()

	// Create security source with your credentials
	sec := &auth.BasicAuthProvider{
		Username: cfg.AuthUsername,
		Password: cfg.AuthPassword,
	}

	httpClient := tracing.NewTracingClient()

	client, err := fileupload.NewClient(
		cfg.ClientUrl,
		sec,
		fileupload.WithClient(httpClient),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	otelProviders, err := tracing.SetupOTelSDK(ctx, "http-file-upload", "v1")
	if err != nil {
		return err
	}

	// handle shutdown properly so nothing leaks
	defer func() {
		err = errors.Join(err, otelProviders.Shutdown(context.Background()))
	}()

	file := http.MultipartFile{}

	client.UploadFile(ctx, &fileupload.UploadFileReq{
		File: file,
	})

	return nil
}
