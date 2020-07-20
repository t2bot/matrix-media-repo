package upload_pipeline

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

func UploadMedia(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, contentType string, fileName string, userId string) (*types.Media, error) {
	defer cleanup.DumpAndCloseStream(r)

	// Step 1: Limit the stream's length
	r = limitStreamLength(ctx, r)

	// Step 2: Buffer the stream
	b, err := bufferStream(ctx, r)
	if err != nil {
		return nil, err
	}

	// Create a utility function for getting at the buffer easily
	stream := func() io.ReadCloser {
		return ioutil.NopCloser(bytes.NewBuffer(b))
	}

	// Step 3: Get a hash of the file
	hash, err := hashFile(ctx, stream())
	if err != nil {
		return nil, err
	}

	// Step 4: Check if the media is quarantined
	err = checkQuarantineStatus(ctx, hash)
	if err != nil {
		return nil, err
	}

	// Step 5: Generate a media ID if we need to
	if mediaId == "" {
		mediaId, err = generateMediaID(ctx, origin)
		if err != nil {
			return nil, err
		}
	}

	// Step 6: De-duplicate the media
	// TODO: Implementation. Check to see if uploading is required, also if the user has already uploaded a copy.

	// Step 7: Cache the file before uploading
	// TODO

	// Step 8: Prepare an async job to persist the media
	// TODO: Implementation. Limit the number of concurrent jobs on this to avoid queue flooding.
	// TODO: Should this be configurable?
	// TODO: Handle partial uploads/incomplete uploads.

	// Step 9: Return the media while it gets persisted
	// TODO

	return nil, errors.New("not yet implemented")
}