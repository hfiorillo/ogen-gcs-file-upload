package auth

import (
	"context"

	"gitlab.com/totalprocessing/file-upload/internal/fileupload"
)

// BasicAuthProvider implements SecuritySource interface
type BasicAuthProvider struct {
	Username string
	Password string
}

func (b *BasicAuthProvider) BasicAuth(ctx context.Context, operationName fileupload.OperationName) (fileupload.BasicAuth, error) {
	return fileupload.BasicAuth{
		Username: b.Username,
		Password: b.Password,
	}, nil
}
