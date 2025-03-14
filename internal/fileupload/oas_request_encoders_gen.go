// Code generated by ogen, DO NOT EDIT.

package fileupload

import (
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/go-faster/errors"

	ht "github.com/ogen-go/ogen/http"
	"github.com/ogen-go/ogen/uri"
)

func encodeUploadFileRequest(
	req *UploadFileReq,
	r *http.Request,
) error {
	const contentType = "multipart/form-data"
	request := req

	q := uri.NewFormEncoder(map[string]string{})
	body, boundary := ht.CreateMultipartBody(func(w *multipart.Writer) error {
		if err := request.File.WriteMultipart("file", w); err != nil {
			return errors.Wrap(err, "write \"file\"")
		}
		if err := q.WriteMultipart(w); err != nil {
			return errors.Wrap(err, "write multipart")
		}
		return nil
	})
	ht.SetCloserBody(r, body, mime.FormatMediaType(contentType, map[string]string{"boundary": boundary}))
	return nil
}
