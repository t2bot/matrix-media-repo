package r0

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/media_handler"
	"github.com/turt2live/matrix-media-repo/storage"
)

// Request:
//   QS: ?filename=
//   Headers: Content-Type
//   Body: <byte[]>
//
// Response:
//   Body: {"content_uri":"mxc://domain.com/media_id"}

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
}

func UploadMedia(w http.ResponseWriter, r *http.Request, db storage.Database) interface{} {
	// TODO: Validate access_token

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = "upload.bin"
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}
	i := strings.Index(contentType, ";")
	if i != -1 {
		contentType = contentType[:i]
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10485760) // TODO: Read max size from config

	tempFile, err := uploadTempFile(r.Body)
	if err != nil {
		return client.InternalServerError(err.Error())
	}

	request := &media_handler.MediaUploadRequest{
		TempLocation: tempFile,
		UploadedBy: "",
		ContentType: contentType,
		DesiredFilename:filename,
		Host:r.Host,
	}

	mxc, err := request.StoreMedia(r.Context(), db)
	if err != nil {
		return client.InternalServerError(err.Error())
	}

	return &MediaUploadedResponse{mxc}
}

func uploadTempFile(reqReader io.ReadCloser) (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "mtx-media-repo")
	if err != nil {
		return "", err
	}

	defer file.Close()

	_, err = io.Copy(file, reqReader)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}