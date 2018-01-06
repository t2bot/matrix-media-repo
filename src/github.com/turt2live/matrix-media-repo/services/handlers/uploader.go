package handlers

import (
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/rcontext"
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

// TODO: These functions should be moved to the media service

func (r MediaUploadRequest) StoreAndGetMxcUri(i rcontext.RequestInfo) (string, error) {
	media, err := r.StoreMedia(i)
	if err != nil {
		return "", err
	}

	return util.MediaToMxc(&media), nil
}

func (r MediaUploadRequest) StoreMediaWithId(mediaId string, info rcontext.RequestInfo) (types.Media, error) {
	info.Log = info.Log.WithFields(logrus.Fields{
		"handlerMediaId": mediaId,
	})

	destination, err := storage.PersistFile(r.Contents, info.Config, info.Context, &info.Db)
	if err != nil {
		return types.Media{}, err
	}

	hash, err := storage.GetFileHash(destination)
	if err != nil {
		os.Remove(destination)
		return types.Media{}, err
	}

	mediaStore := info.Db.GetMediaStore(info.Context, info.Log)

	records, err := mediaStore.GetByHash(hash)
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
				info.Log.Info("Exact media duplicate found, returning unaltered media record")
				return media, nil
			}

			if media.Origin == r.Host {
				info.Log.Info("Media duplicate found, assigning a new media ID for new origin")
				// Generate a new ID for this upload
				media.MediaId = GenerateMediaId()
				break
			}
		}

		info.Log.Info("Duplicate media found, generating new record using existing file")

		media.Origin = r.Host
		media.UserId = r.UploadedBy
		media.UploadName = r.DesiredFilename
		media.ContentType = r.ContentType
		media.CreationTs = time.Now().UnixNano() / 1000000

		err = mediaStore.Insert(&media)
		if err != nil {
			return types.Media{}, err
		}

		return media, nil
	}

	info.Log.Info("Persisting unique media record")

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

	err = mediaStore.Insert(media)
	if err != nil {
		os.Remove(destination)
		return types.Media{}, err
	}

	return *media, nil
}

func (r MediaUploadRequest) StoreMedia(i rcontext.RequestInfo) (types.Media, error) {
	return r.StoreMediaWithId(GenerateMediaId(), i)
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
