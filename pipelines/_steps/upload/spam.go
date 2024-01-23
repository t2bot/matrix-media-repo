package upload

import (
	"io"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/plugins"
)

type FileMetadata struct {
	Name        string
	ContentType string
	UserId      string
	Origin      string
	MediaId     string
}

type SpamResponse struct {
	Err    error
	IsSpam bool
}

func CheckSpamAsync(ctx rcontext.RequestContext, reader io.Reader, metadata FileMetadata) chan SpamResponse {
	opChan := make(chan SpamResponse)
	go func() {
		//goland:noinspection GoUnhandledErrorResult
		defer io.Copy(io.Discard, reader) // we need to flush the reader as we might end up blocking the upload

		spam, err := plugins.CheckForSpam(reader, metadata.Name, metadata.ContentType, metadata.UserId, metadata.Origin, metadata.MediaId)
		go func() {
			// run async to avoid deadlock
			opChan <- SpamResponse{
				Err:    err,
				IsSpam: spam,
			}
		}()
	}()
	return opChan
}
