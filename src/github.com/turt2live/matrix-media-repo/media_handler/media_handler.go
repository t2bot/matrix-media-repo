package media_handler

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaUploadRequest struct {
	Contents io.Reader
	DesiredFilename string
	UploadedBy string
	Host string
	ContentType string
}

func (r MediaUploadRequest) StoreMedia(ctx context.Context, c config.MediaRepoConfig, db storage.Database) (string, error) {
	destination, err := storage.PersistFile(ctx, r.Contents, c, db)
	if err != nil {
		return "", err
	}

	hash, err := storage.GetFileHash(destination)
	if err != nil {
		os.Remove(destination)
		return "", err
	}

	records, err := db.GetMediaByHash(ctx, hash)
	if err != nil {
		os.Remove(destination)
		return "", err
	}
	if len(records) > 0 {
		// We can delete the media: It's already duplicated at this point
		os.Remove(destination)

		var media types.Media

		// Try and find an already-existing media item for this host
		for i := 0; i < len(records); i++ {
			media = records[i]

			// If the media is exactly the same, just return it
			if IsMediaSame(media, r) {
				return util.MediaToMxc(&media), nil
			}

			if media.Origin == r.Host {
				// Generate a new ID for this upload
				media.MediaId = GenerateMediaId()
				break
			}
		}

		media.Origin = r.Host
		media.UserId = r.UploadedBy
		media.UploadName = r.DesiredFilename
		media.ContentType = r.ContentType
		media.CreationTs = time.Now().UnixNano() / 1000000

		err = db.InsertMedia(ctx, &media)
		if err != nil {
			return "", err
		}

		return util.MediaToMxc(&media), nil
	}

	fileSize, err := util.FileSize(destination)
	if err != nil {
		return "", err
	}

	media := &types.Media{
		Origin:      r.Host,
		MediaId:     GenerateMediaId(),
		UploadName:  r.DesiredFilename,
		ContentType: r.ContentType,
		UserId:      r.UploadedBy,
		Sha256Hash:  hash,
		SizeBytes:   fileSize,
		Location:    destination,
		CreationTs:  time.Now().UnixNano() / 1000000,
	}

	err = db.InsertMedia(ctx, media)
	if err != nil {
		os.Remove(destination)
		return "", err
	}

	return util.MediaToMxc(media), nil
}

func GenerateMediaId() string {
	str, err := util.GenerateRandomString(64)
	if err != nil {
		panic(err)
	}

	return str
}

func IsMediaSame(media types.Media, r MediaUploadRequest) bool {
	originSame := media.Origin == r.Host
	nameSame := media.UploadName == r.DesiredFilename
	userSame := media.UserId == r.UploadedBy
	typeSame := media.ContentType == r.ContentType

	return originSame && nameSame && userSame && typeSame
}