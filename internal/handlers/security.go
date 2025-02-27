package handlers

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"golang.org/x/crypto/bcrypt"
)

var _ fileupload.SecurityHandler = (*SecurityHandler)(nil)

type SecurityHandler struct {
	logger       *slog.Logger
	AuthUsername string
	AuthPassword string
}

// This allows us to mock the client for testing
type SecurityClient interface {
	HandleBasicAuth(ctx context.Context, operationName fileupload.OperationName, auth fileupload.BasicAuth) (context.Context, error)
}

// NewUploadHandler creates a new security handler
func NewSecurityHandler(logger *slog.Logger, username, password string) *SecurityHandler {
	return &SecurityHandler{
		logger:       logger,
		AuthUsername: username,
		AuthPassword: password,
	}
}

// HandleBasicAuth handles basic authentication
func (h *SecurityHandler) HandleBasicAuth(ctx context.Context, operationName fileupload.OperationName, auth fileupload.BasicAuth) (context.Context, error) {
	startTime := time.Now()

	// hashed, exists := validCredentials[auth.Username]
	// if !exists {
	// 	return ctx, errors.New("error credentials invalid")
	// }

	if err := bcrypt.CompareHashAndPassword([]byte(auth.Password), []byte(h.AuthPassword)); err != nil {
		return ctx, errors.New("error credentials invalid")
	}

	h.logger.Info("authenticated succesfully",
		"operation", operationName,
		"username", auth.Username,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return context.WithValue(ctx, "user", h.AuthUsername), nil
}
