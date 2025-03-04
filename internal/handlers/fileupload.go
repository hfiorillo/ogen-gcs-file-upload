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
		case strings.Contains(err.Error(), "invalid file type"),
			strings.Contains(err.Error(), "file size exceeds"):
			return &fileupload.UploadFileBadRequest{}, err
		default:
			return &fileupload.UploadFileInternalServerError{}, err
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

// TODO: whats this for?
func (h *UploadHandler) NewError(ctx context.Context, err error) *fileupload.ErrorStatusCodeWithHeaders {
	return &fileupload.ErrorStatusCodeWithHeaders{}
}
