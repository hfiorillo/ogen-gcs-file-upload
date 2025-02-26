package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/gcs"
	"gitlab.com/totalprocessing/file-upload/internal/handlers"
	"gitlab.com/totalprocessing/file-upload/internal/logs"
)

const filename string = "test-file"

func main() {
	ctx := context.Background()

	logger := logs.NewPrettyLogger()

	// Initialize GCS client
	var err error
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer gcsClient.Close()

	// Create security handler?
	sec := handlers.NewSecurityHandler(logger, "authentication")
	h := handlers.NewUploadHandler(logger, filename, gcs.GcsClient{
		GcsClient: gcsClient,
	})

	// Create server
	fileUploadServer, err := fileupload.NewServer(h, sec)
	if err != nil {
		logger.Error("failed to create server", "error", err)
		panic(fmt.Sprintf("failed to create server: %v", err))
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
		logger.Error("server error", "error", err)
	case sig := <-shutdown:
		logger.Info("shutdown", "status", "shutdown started", "signal", sig)
		defer logger.Info("shutdown", "status", "shutdown complete", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 2)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			server.Close()
			logger.Error("server could not stop gracefully", "error", err)
		}
	}
}
