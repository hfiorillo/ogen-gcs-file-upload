package gcs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	ogenhttp "github.com/ogen-go/ogen/http"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
)

// TODO: move to env vars
const (
	maxUploadSize = 10 * 1024 * 1024 // 10MB
)

type GcsConfig struct {
	GcsProject    string
	GcsLocation   string
	GcsBucketName string
}

// Allowed MIME types for file uploads
var allowedTypes = map[string]bool{
	"text/csv":        true,
	"application/csv": true,
	// https://learn.microsoft.com/en-us/previous-versions/office/office-2007-resource-kit/ee309278(v=office.12)?redirectedfrom=MSDN
	"vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
}

type GcsClient struct {
	GcsConfig GcsConfig
	Logger    *slog.Logger
	GcsClient *storage.Client
}

// NewGcsClient creates a new GCS client
func NewGcsClient(ctx context.Context) (*storage.Client, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	return client, nil
}

// UploadToGcs handles file uploads to Google Cloud Storage
func (g *GcsClient) UploadToGcs(ctx context.Context, filename string, contentType string, file ogenhttp.MultipartFile) (*fileupload.UploadResponse, error) {

	// Validate and sanitize input
	filename = sanitizeFilename(filename)
	if err := validateFile(file); err != nil {
		g.Logger.Error("file failed validation", "error", err)
		return nil, err
	}

	// Create GCS object writer
	obj := g.GcsClient.Bucket(g.GcsConfig.GcsBucketName).Object(filename)
	w := obj.NewWriter(ctx)
	defer w.Close()

	// Set content type metadata
	w.ObjectAttrs.ContentType = contentType

	// Upload file with retry logic
	size, err := uploadWithRetry(w, file.File)
	if err != nil {
		return nil, fmt.Errorf("gcs upload failed for %s: %w", filename, err)
	}

	return &fileupload.UploadResponse{
		Filename:   filename,
		Gcspath:    fileupload.NewOptString(fmt.Sprintf("gs://%s/%s", g.GcsConfig.GcsBucketName, filename)),
		FileSize:   size,
		UploadTime: time.Now().UTC(),
	}, nil
}

// validateFile checks file size and type
func validateFile(file ogenhttp.MultipartFile) error {
	if file.Size > maxUploadSize {
		return fmt.Errorf("file size exceeds 10MB limit: %d bytes", file.Size)
	}

	contentType, err := detectContentType(file)
	if err != nil {
		return fmt.Errorf("could not detect file type")
	}

	if !isAllowedType(contentType) {
		return fmt.Errorf("invalid file type: %s. allowed types: %v", contentType, allowedTypes)
	}

	return nil
}

// isAllowedType checks if the content type is permitted
func isAllowedType(contentType string) bool {
	fmt.Println("Extracted Content-Type", "contentType", contentType)
	baseType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	fmt.Println("baseType", baseType)
	return allowedTypes[baseType]
}

// Infer MIME type from filename
func inferMimeType(filename string, detectedType string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return detectedType
	}

	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		return mimeType
	}

	return detectedType
}

// Better way of detecting mime type for xlsx
func detectContentType(file ogenhttp.MultipartFile) (string, error) {
	buf := make([]byte, 512)
	_, err := file.File.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	detectedType := http.DetectContentType(buf)
	contentType := inferMimeType(file.Name, detectedType)
	return contentType, nil
}

// uploadWithRetry implements retry logic for GCS uploads
func uploadWithRetry(w io.Writer, file io.Reader) (int64, error) {
	const maxRetries = 3
	var backoff = []time.Duration{
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
	}

	var size int64
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		size, err = io.Copy(w, file)
		if err == nil {
			return size, nil
		}

		if attempt < maxRetries {
			time.Sleep(backoff[attempt])
		}
	}

	return 0, fmt.Errorf("upload failed after %d retries: %w", maxRetries, err)
}

// sanitizeFilename ensures safe filenames
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "..", "")
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return '_'
		}
		return r
	}, name)

	return strings.TrimSpace(name)
}
