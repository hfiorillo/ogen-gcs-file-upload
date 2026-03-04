package gcs

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	ogenhttp "github.com/ogen-go/ogen/http"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
)

const defaultMaxUploadSizeBytes int64 = 10 * 1024 * 1024 // 10MB

var (
	ErrFileTooLarge    = errors.New("file size exceeds upload limit")
	ErrInvalidFile     = errors.New("invalid file")
	ErrInvalidFileType = errors.New("invalid file type")
)

type GcsConfig struct {
	GcsProject         string
	GcsLocation        string
	GcsBucketName      string
	MaxUploadSizeBytes int64
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

func (c GcsConfig) maxUploadSize() int64 {
	if c.MaxUploadSizeBytes > 0 {
		return c.MaxUploadSizeBytes
	}
	return defaultMaxUploadSizeBytes
}

// UploadToGcs handles file uploads to Google Cloud Storage
func (g *GcsClient) UploadToGcs(ctx context.Context, filename string, file ogenhttp.MultipartFile) (*fileupload.UploadResponse, error) {
	filename = sanitizeFilename(filename)
	if filename == "" {
		return nil, fmt.Errorf("%w: empty filename", ErrInvalidFile)
	}

	fileBytes, err := readFileWithLimit(file.File, file.Size, g.GcsConfig.maxUploadSize())
	if err != nil {
		g.Logger.Error("file failed validation", "filename", filename, "error", err)
		return nil, err
	}

	contentType, err := detectContentType(filename, fileBytes)
	if err != nil {
		g.Logger.Error("content type detection failed", "filename", filename, "error", err)
		return nil, err
	}

	obj := g.GcsClient.Bucket(g.GcsConfig.GcsBucketName).Object(filename)
	size, err := uploadWithRetry(fileBytes, func(reader io.Reader) error {
		w := obj.NewWriter(ctx)
		w.ContentType = contentType

		if _, copyErr := io.Copy(w, reader); copyErr != nil {
			_ = w.Close()
			return copyErr
		}

		if closeErr := w.Close(); closeErr != nil {
			return closeErr
		}

		return nil
	})
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

func readFileWithLimit(reader io.Reader, declaredSize int64, maxSize int64) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: nil file reader", ErrInvalidFile)
	}

	if declaredSize > maxSize {
		return nil, fmt.Errorf("%w: got %d bytes, limit %d bytes", ErrFileTooLarge, declaredSize, maxSize)
	}

	limitedReader := io.LimitReader(reader, maxSize+1)
	payload, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("%w: failed reading payload: %v", ErrInvalidFile, err)
	}

	if int64(len(payload)) > maxSize {
		return nil, fmt.Errorf("%w: got %d bytes, limit %d bytes", ErrFileTooLarge, len(payload), maxSize)
	}

	if len(payload) == 0 {
		return nil, fmt.Errorf("%w: empty payload", ErrInvalidFile)
	}

	return payload, nil
}

func detectContentType(filename string, payload []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	sniff := payload
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}

	switch ext {
	case ".csv":
		if !looksLikeCSV(sniff) {
			return "", fmt.Errorf("%w: invalid csv payload", ErrInvalidFileType)
		}
		return "text/csv", nil
	case ".xlsx":
		if !bytes.HasPrefix(sniff, []byte{0x50, 0x4B, 0x03, 0x04}) {
			return "", fmt.Errorf("%w: invalid xlsx payload", ErrInvalidFileType)
		}
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
	default:
		return "", fmt.Errorf("%w: unsupported extension %q", ErrInvalidFileType, ext)
	}
}

func looksLikeCSV(sniff []byte) bool {
	detectedType := http.DetectContentType(sniff)
	if detectedType == "application/json" || detectedType == "application/zip" {
		return false
	}

	reader := csv.NewReader(bytes.NewReader(sniff))
	reader.FieldsPerRecord = -1
	_, err := reader.Read()
	return err == nil || errors.Is(err, io.EOF)
}

// uploadWithRetry retries upload with a fresh reader each time.
func uploadWithRetry(payload []byte, uploadFn func(io.Reader) error) (int64, error) {
	const maxAttempts = 3
	backoff := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond}

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = uploadFn(bytes.NewReader(payload))
		if err == nil {
			return int64(len(payload)), nil
		}

		if attempt < maxAttempts {
			time.Sleep(backoff[attempt-1])
		}
	}

	return 0, fmt.Errorf("upload failed after %d attempts: %w", maxAttempts, err)
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
