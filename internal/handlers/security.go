package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"golang.org/x/crypto/bcrypt"
)

var _ fileupload.SecurityHandler = (*SecurityHandler)(nil)

type contextKey string

const userContextKey contextKey = "user"

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

	if !constantTimeEqual(auth.Username, h.AuthUsername) || !h.passwordMatches(auth.Password) {
		h.logger.Warn("authentication unsuccessful",
			"username", auth.Username,
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return ctx, errors.New("error credentials invalid")
	}

	h.logger.Info("authenticated successfully",
		"operation", operationName,
		"username", auth.Username,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return context.WithValue(ctx, userContextKey, h.AuthUsername), nil
}

func (h *SecurityHandler) passwordMatches(password string) bool {
	if strings.HasPrefix(h.AuthPassword, "$2a$") ||
		strings.HasPrefix(h.AuthPassword, "$2b$") ||
		strings.HasPrefix(h.AuthPassword, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(h.AuthPassword), []byte(password)) == nil
	}

	return constantTimeEqual(password, h.AuthPassword)
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
