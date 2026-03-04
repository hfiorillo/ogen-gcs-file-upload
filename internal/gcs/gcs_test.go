package gcs

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadFileWithLimitRejectsLargeFile(t *testing.T) {
	_, err := readFileWithLimit(strings.NewReader("01234567890"), int64(len("01234567890")), 10)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFileTooLarge)
}

func TestDetectContentTypeCSV(t *testing.T) {
	contentType, err := detectContentType("sample.csv", []byte("name,age\nAlice,30\n"))
	require.NoError(t, err)
	require.Equal(t, "text/csv", contentType)
}

func TestDetectContentTypeRejectsJSONRenamedToCSV(t *testing.T) {
	_, err := detectContentType("sample.csv", []byte(`{"name":"Alice","age":30}`))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidFileType)
}

func TestDetectContentTypeXLSX(t *testing.T) {
	contentType, err := detectContentType("sample.xlsx", append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("xlsx-data")...))
	require.NoError(t, err)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", contentType)
}

func TestDetectContentTypeRejectsInvalidXLSXPayload(t *testing.T) {
	_, err := detectContentType("sample.xlsx", []byte("not-a-zip"))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidFileType)
}

func TestUploadWithRetryRetriesWithFreshReader(t *testing.T) {
	payload := []byte("test-payload")
	calls := 0

	size, err := uploadWithRetry(payload, func(r io.Reader) error {
		calls++
		b, readErr := io.ReadAll(r)
		require.NoError(t, readErr)
		require.Equal(t, payload, b)

		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, int64(len(payload)), size)
	require.Equal(t, 3, calls)
}

func TestUploadWithRetryFailsAfterMaxAttempts(t *testing.T) {
	calls := 0

	_, err := uploadWithRetry([]byte("x"), func(r io.Reader) error {
		calls++
		return errors.New("always fails")
	})

	require.Error(t, err)
	require.Equal(t, 3, calls)
}
