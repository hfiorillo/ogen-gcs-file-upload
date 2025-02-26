package handlers

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/gcs"
)

// This ensures my handler follows the spec
var _ fileupload.Handler = (*UploadHandler)(nil)

// UploadHandler implements the generated Handler interface for uploading a file
type UploadHandler struct {
	logger    *slog.Logger
	Filename  string
	GcsClient gcs.GcsClient
}

// This allows us to mock the client for testing
type FileUploadClient interface {
	UploadFile(ctx context.Context, req *fileupload.UploadFileReq) (fileupload.UploadFileRes, error)
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(logger *slog.Logger, filename string, gcsClient gcs.GcsClient) *UploadHandler {
	return &UploadHandler{
		logger:    logger,
		Filename:  filename,
		GcsClient: gcsClient,
	}
}

// UploadFile handles file upload requests\
// TODO: bug when multiple files are uploaded
// TODO: check file type
func (h *UploadHandler) UploadFile(ctx context.Context, req *fileupload.UploadFileReq) (fileupload.UploadFileRes, error) {

	startTime := time.Now()

	response, err := h.GcsClient.UploadToGcs(ctx, req.File.Name, req.File.Header.Get("Content-Type"), req.File)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid file type"),
			strings.Contains(err.Error(), "file size exceeds"):
			return &fileupload.UploadFileBadRequest{Error: err.Error()}, nil
		default:
			return &fileupload.UploadFileInternalServerError{Error: err.Error()}, nil
		}
	}

	h.logger.Info("file uploaded successfully",
		"filename", req.File.Name,
		"size", req.File.Size,
		// "gcs filepath", req.GcsPath.Value,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return &fileupload.UploadResponseHeaders{
		Response: *response,
	}, nil
}
