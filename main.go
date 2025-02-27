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
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/pkg/errors"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/gcs"
	"gitlab.com/totalprocessing/file-upload/internal/handlers"
	"gitlab.com/totalprocessing/file-upload/internal/logs"
)

const filename string = "test-file"

func main() {
	logger := logs.NewPrettyLogger()
	if err := run(logger); err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer gcsClient.Close()

	sec := handlers.NewSecurityHandler(logger, "authentication")
	h := handlers.NewUploadHandler(logger, filename, gcs.GcsClient{
		Logger:    logger,
		GcsClient: gcsClient,
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

	// // Wrap with middleware to log errors
	// wrappedServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	recorder := &statusRecorder{w, 200}
	// 	fileUploadServer.ServeHTTP(recorder, r)
	// 	if recorder.status >= 400 {
	// 		logger.Error("http error response",
	// 			"status", recorder.status,
	// 			"path", r.URL.Path,
	// 			"method", r.Method,
	// 		)
	// 	}
	// })

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

// type statusRecorder struct {
// 	http.ResponseWriter
// 	status int
// }

// func (r *statusRecorder) WriteHeader(status int) {
// 	r.status = status
// 	r.ResponseWriter.WriteHeader(status)
// }
