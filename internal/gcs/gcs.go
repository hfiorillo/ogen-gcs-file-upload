package gcs

import (
	"context"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/ogen-go/ogen/http"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
)

const (
	bucketName    = "dwh-test-upload-file"
	maxUploadSize = 10 * 1024 * 1024
	allowedTypes  = "text/plain,application/pdf,image/"
)

type GcsClient struct {
	GcsClient *storage.Client
}

// New storage gcs client
func NewGcsClient(ctx context.Context) (*storage.Client, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return &storage.Client{}, err
	}
	return client, nil
}

// Actually uploads the file to GCS
func (g *GcsClient) UploadToGcs(ctx context.Context, filename string, contentType string, file http.MultipartFile) (*fileupload.UploadResponse, error) {

	filename = sanitizeFilename(filename)

	if file.Size > maxUploadSize {
		return nil, fmt.Errorf("file size exceeds 10MB limit")
	}

	// 2. Validate content type
	if !isAllowedType(contentType) {
		return nil, fmt.Errorf("invalid file type: %s", contentType)
	}
	bucket := g.GcsClient.Bucket(bucketName)
	obj := bucket.Object(filename)
	w := obj.NewWriter(ctx)
	defer w.Close()

	size, err := io.Copy(w, file.File)
	if err != nil {
		return nil, fmt.Errorf("gcs upload failed: %w", err)
	}

	// const maxRetries = 3
	// var backoff = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	// for i := 0; ; i++ {
	// 	_, err = io.Copy(w, file)
	// 	if err == nil {
	// 		break
	// 	}
	// 	if i >= maxRetries {
	// 		return nil, fmt.Errorf("upload failed after %d retries: %w", maxRetries, err)
	// 	}
	// 	time.Sleep(backoff[i])
	// }

	return &fileupload.UploadResponse{
		Filename: fileupload.OptString{Value: filename},
		// GcsPath:    fileupload.OptURI{Value: obj.ObjectName()},
		Size:       fileupload.OptInt64{Value: size},
		UploadedAt: fileupload.OptDateTime{Value: time.Now().UTC()},
	}, nil
}

// Helper functions
func isAllowedType(contentType string) bool {
	baseType := strings.Split(mime.FormatMediaType(contentType, nil), ";")[0]
	return strings.HasPrefix(baseType, allowedTypes) ||
		baseType == "application/pdf" ||
		baseType == "text/plain"
}

func sanitizeFilename(name string) string {
	return strings.ReplaceAll(name, "..", "") // Basic sanitation
}
