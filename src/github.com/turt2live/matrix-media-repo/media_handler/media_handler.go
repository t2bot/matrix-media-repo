package media_handler

import (
	"context"
	"os"
	"time"

	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaUploadRequest struct {
	TempLocation string
	DesiredFilename string
	UploadedBy string
	Host string
	ContentType string
}

func (r MediaUploadRequest) StoreMedia(ctx context.Context, db storage.Database) (string, error) {
	hash, err := storage.GetFileHash(r.TempLocation)
	if err != nil {
		return "", err
	}

	// TODO: Dedupe media here

	destination, err := storage.PersistTempFile(r.TempLocation)
	if err != nil {
		return "", err
	}

	f, err := os.Open(destination)
	if err != nil {
		return "", err
	}

	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	fileSize := fi.Size()

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