package gcs

import (
	"bytes"
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
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"application/json": true,
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
func (g *GcsClient) UploadToGcs(ctx context.Context, filename string, file ogenhttp.MultipartFile) (*fileupload.UploadResponse, error) {

	// Validate and sanitize input
	filename = sanitizeFilename(filename)
	if err := g.validateFile(file); err != nil {
		g.Logger.Error("file failed validation", "error", err)
		return nil, err
	}

	// Create GCS object writer
	obj := g.GcsClient.Bucket(g.GcsConfig.GcsBucketName).Object(filename)
	w := obj.NewWriter(ctx)
	defer w.Close()

	// Set content type metadata
	// w.ObjectAttrs.ContentType = contentType

	// Upload file with retry logic
	size, err := uploadWithRetry(w, file.File)
	if err != nil {
		return nil, fmt.Errorf("gcs upload failed for %s: %w", filename, err)
	}

	return &fileupload.UploadResponse{
		Filename:   filename,
		Bucket:     g.GcsConfig.GcsBucketName,
		Gcspath:    fileupload.NewOptString(fmt.Sprintf("gs://%s/%s", g.GcsConfig.GcsBucketName, filename)),
		FileSize:   size,
		UploadTime: time.Now().UTC(),
	}, nil
}

func (g *GcsClient) validateFile(file ogenhttp.MultipartFile) error {
	g.Logger.Info("File details",
		"filename", file.Name,
		"file", fmt.Sprintf("%+v", file),
		"file.Size", file.Size)

	contentType, err := detectContentType(file)
	if err != nil {
		g.Logger.Error("content type detection failed",
			"filename", file.Name,
			"error", err)
		return err
	}

	g.Logger.Info("file content type detected",
		"filename", file.Name,
		"contentType", contentType,
	)

	if !allowedTypes[contentType] {
		return fmt.Errorf("invalid file type: %s. allowed types: %v", contentType, allowedTypes)
	}

	return nil
}

// // isAllowedType checks if the content type is permitted
// func isAllowedType(contentType string) bool {
// 	// baseType, _, err := mime.ParseMediaType(contentType)
// 	// if err != nil {
// 	// 	return false
// 	// }
// 	return allowedTypes[contentType]
// }

// Better way of detecting mime type for complex file types
func detectContentType(file ogenhttp.MultipartFile) (string, error) {
	// Validate input
	if file.File == nil {
		return "", fmt.Errorf("nil file reader")
	}

	// Validate filename
	if file.Name == "" {
		return "", fmt.Errorf("empty filename")
	}

	ext := filepath.Ext(file.Name)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType, nil
		}
	}

	// Limit read to prevent potential DoS via large file reads
	lr := io.LimitReader(file.File, 512)
	buf := make([]byte, 512)

	n, err := lr.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Detect content type from bytes
	detectedType := http.DetectContentType(buf[:n])

	// Special handling for CSV files
	if strings.Contains(strings.ToLower(file.Name), ".csv") {
		// Check for typical CSV content characteristics
		csvContent := string(buf[:n])
		if strings.Contains(csvContent, ",") || strings.Contains(csvContent, ";") {
			return "text/csv", nil
		}
	}

	// Special handling for problematic detections
	switch detectedType {
	case "text/plain":
		if strings.Contains(strings.ToLower(file.Name), ".csv") {
			return "text/csv", nil
		}
	case "application/zip":
		// Strict signature checks for Excel files
		if bytes.HasPrefix(buf, []byte{0x50, 0x4B, 0x03, 0x04}) {
			if strings.Contains(strings.ToLower(file.Name), ".xlsx") {
				detectedType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			}
		}
	}

	// Final type validation
	if !allowedTypes[detectedType] {
		return "", fmt.Errorf("unauthorized file type: %s", detectedType)
	}

	// If no specific type found, use the detected type
	return detectedType, nil
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
