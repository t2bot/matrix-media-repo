package media_handler

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaUploadRequest struct {
	Contents        io.Reader
	DesiredFilename string
	UploadedBy      string
	Host            string
	ContentType     string
}

func (r MediaUploadRequest) StoreAndGetMxcUri(ctx context.Context, c config.MediaRepoConfig, db storage.Database, log *logrus.Entry) (string, error) {
	media, err := r.StoreMedia(ctx, c, db, log)
	if err != nil {
		return "", err
	}

	return util.MediaToMxc(&media), nil
}

func (r MediaUploadRequest) StoreMediaWithId(ctx context.Context, mediaId string, c config.MediaRepoConfig, db storage.Database, log *logrus.Entry) (types.Media, error) {
	log = log.WithFields(logrus.Fields{
		"handlerMediaId": mediaId,
	})

	destination, err := storage.PersistFile(ctx, r.Contents, c, db)
	if err != nil {
		return types.Media{}, err
	}

	hash, err := storage.GetFileHash(destination)
	if err != nil {
		os.Remove(destination)
		return types.Media{}, err
	}

	records, err := db.GetMediaByHash(ctx, hash)
	if err != nil {
		os.Remove(destination)
		return types.Media{}, err
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
				log.Info("Exact media duplicate found, returning unaltered media record")
				return media, nil
			}

			if media.Origin == r.Host {
				log.Info("Media duplicate found, assigning a new media ID for new origin")
				// Generate a new ID for this upload
				media.MediaId = GenerateMediaId()
				break
			}
		}

		log.Info("Duplicate media found, generating new record using existing file")

		media.Origin = r.Host
		media.UserId = r.UploadedBy
		media.UploadName = r.DesiredFilename
		media.ContentType = r.ContentType
		media.CreationTs = time.Now().UnixNano() / 1000000

		err = db.InsertMedia(ctx, &media)
		if err != nil {
			return types.Media{}, err
		}

		return media, nil
	}

	log.Info("Persisting unique media record")

	fileSize, err := util.FileSize(destination)
	if err != nil {
		return types.Media{}, err
	}

	media := &types.Media{
		Origin:      r.Host,
		MediaId:     mediaId,
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
		return types.Media{}, err
	}

	return *media, nil
}

func (r MediaUploadRequest) StoreMedia(ctx context.Context, c config.MediaRepoConfig, db storage.Database, log *logrus.Entry) (types.Media, error) {
	return r.StoreMediaWithId(ctx, GenerateMediaId(), c, db, log)
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
