package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/stretchr/testify/require"
	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
	"gitlab.com/totalprocessing/file-upload/internal/gcs"
)

func TestNewErrorDefaultsToInternalServerError(t *testing.T) {
	handler := NewUploadHandler(newDiscardLogger(), gcsClientZero())

	res := handler.NewError(context.Background(), errors.New("boom"))
	require.Equal(t, http.StatusInternalServerError, res.StatusCode)
	require.Equal(t, int32(http.StatusInternalServerError), res.Response.Code)
	require.Equal(t, "internal server error", res.Response.Message)
	require.Equal(t, "*", res.AccessControlAllowOrigin.Value)
}

func TestNewErrorMapsSecurityErrorToUnauthorized(t *testing.T) {
	handler := NewUploadHandler(newDiscardLogger(), gcsClientZero())

	res := handler.NewError(context.Background(), &ogenerrors.SecurityError{
		OperationContext: ogenerrors.OperationContext{
			Name: fileupload.UploadFileOperation,
			ID:   "uploadFile",
		},
		Security: "BasicAuth",
		Err:      errors.New("invalid credentials"),
	})

	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, int32(http.StatusUnauthorized), res.Response.Code)
	require.Equal(t, "unauthorized", res.Response.Message)
}

func TestNewErrorMapsDecodeErrorToBadRequest(t *testing.T) {
	handler := NewUploadHandler(newDiscardLogger(), gcsClientZero())

	res := handler.NewError(context.Background(), &ogenerrors.DecodeRequestError{
		OperationContext: ogenerrors.OperationContext{
			Name: fileupload.UploadFileOperation,
			ID:   "uploadFile",
		},
		Err: errors.New("bad body"),
	})

	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.Equal(t, int32(http.StatusBadRequest), res.Response.Code)
	require.Equal(t, "bad request", res.Response.Message)
}

func gcsClientZero() gcs.GcsClient {
	return gcs.GcsClient{}
}
