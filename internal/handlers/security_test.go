package handlers

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"golang.org/x/crypto/bcrypt"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandleBasicAuthWithHashedPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.MinCost)
	require.NoError(t, err)

	handler := NewSecurityHandler(newDiscardLogger(), "testuser", string(hash))

	_, err = handler.HandleBasicAuth(context.Background(), fileupload.UploadFileOperation, fileupload.BasicAuth{
		Username: "testuser",
		Password: "testpass",
	})
	require.NoError(t, err)
}

func TestHandleBasicAuthRejectsWrongUsername(t *testing.T) {
	handler := NewSecurityHandler(newDiscardLogger(), "testuser", "testpass")

	_, err := handler.HandleBasicAuth(context.Background(), fileupload.UploadFileOperation, fileupload.BasicAuth{
		Username: "wronguser",
		Password: "testpass",
	})
	require.Error(t, err)
}

func TestHandleBasicAuthRejectsWrongPassword(t *testing.T) {
	handler := NewSecurityHandler(newDiscardLogger(), "testuser", "testpass")

	_, err := handler.HandleBasicAuth(context.Background(), fileupload.UploadFileOperation, fileupload.BasicAuth{
		Username: "testuser",
		Password: "wrongpass",
	})
	require.Error(t, err)
}

func TestHandleBasicAuthWithPlaintextPassword(t *testing.T) {
	handler := NewSecurityHandler(newDiscardLogger(), "testuser", "testpass")

	_, err := handler.HandleBasicAuth(context.Background(), fileupload.UploadFileOperation, fileupload.BasicAuth{
		Username: "testuser",
		Password: "testpass",
	})
	require.NoError(t, err)
}
