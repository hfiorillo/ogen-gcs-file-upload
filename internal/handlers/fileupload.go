package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ogen-go/ogen/ogenerrors"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/gcs"
)

// This ensures my handler follows the spec
var _ fileupload.Handler = (*UploadHandler)(nil)

// UploadHandler implements the generated Handler interface for uploading a file
type UploadHandler struct {
	logger    *slog.Logger
	GcsClient gcs.GcsClient
}

// This allows us to mock the client for testing
type FileUploadClient interface {
	UploadFile(ctx context.Context, req *fileupload.UploadFileReq) (fileupload.UploadFileRes, error)
	NewError(ctx context.Context, err error) *fileupload.ErrorStatusCodeWithHeaders
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(logger *slog.Logger, gcsClient gcs.GcsClient) *UploadHandler {
	return &UploadHandler{
		logger:    logger,
		GcsClient: gcsClient,
	}
}

// UploadFile handles file upload requests
// TODO: bug when multiple files are uploaded
func (h *UploadHandler) UploadFile(ctx context.Context, req *fileupload.UploadFileReq) (fileupload.UploadFileRes, error) {
	startTime := time.Now()
	response, err := h.GcsClient.UploadToGcs(ctx, req.File.Name, req.File)
	if err != nil {
		switch {
		case errors.Is(err, gcs.ErrInvalidFileType),
			errors.Is(err, gcs.ErrFileTooLarge),
			errors.Is(err, gcs.ErrInvalidFile):
			return &fileupload.UploadFileBadRequest{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
				Details: []string{},
			}, nil
		default:
			h.logger.ErrorContext(ctx, "failed to upload file", "error", err)
			return &fileupload.UploadFileInternalServerError{
				Code:    http.StatusInternalServerError,
				Message: "failed to upload file",
				Details: []string{},
			}, nil
		}
	}

	h.logger.Info("file uploaded successfully",
		"filename", response.Filename,
		"size", response.FileSize,
		"gcsPath", response.Gcspath.Value,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return &fileupload.UploadResponseHeaders{
		AccessControlAllowOrigin: fileupload.NewOptString("*"),
		Response:                 *response,
	}, nil
}

func (h *UploadHandler) NewError(ctx context.Context, err error) *fileupload.ErrorStatusCodeWithHeaders {
	statusCode := http.StatusInternalServerError
	message := "internal server error"

	var securityErr *ogenerrors.SecurityError
	if errors.As(err, &securityErr) || errors.Is(err, ogenerrors.ErrSecurityRequirementIsNotSatisfied) {
		statusCode = http.StatusUnauthorized
		message = "unauthorized"
	}

	var decodeErr *ogenerrors.DecodeRequestError
	if errors.As(err, &decodeErr) {
		statusCode = http.StatusBadRequest
		message = "bad request"
	}

	if statusCode >= http.StatusInternalServerError {
		h.logger.ErrorContext(ctx, "unhandled request error", "error", err)
	}

	return &fileupload.ErrorStatusCodeWithHeaders{
		StatusCode:               statusCode,
		AccessControlAllowOrigin: fileupload.NewOptString("*"),
		Response: fileupload.Error{
			Code:    int32(statusCode),
			Message: message,
			Details: []string{},
		},
	}
}
