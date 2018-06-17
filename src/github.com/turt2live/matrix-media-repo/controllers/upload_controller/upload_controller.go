package upload_controller

import (
	"context"
	"io"
	"os"
	"strconv"

	"github.com/ryanuber/go-glob"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

const NoApplicableUploadUser = ""

func IsRequestTooLarge(contentLength int64, contentLengthHeader string) bool {
	if config.Get().Uploads.MaxSizeBytes <= 0 {
		return false
	}
	if contentLength >= 0 {
		return contentLength > config.Get().Uploads.MaxSizeBytes
	}
	if contentLengthHeader != "" {
		parsed, err := strconv.ParseInt(contentLengthHeader, 10, 64)
		if err != nil {
			logrus.Warn("Invalid content length header given; assuming too large. Value received: " + contentLengthHeader)
			return true // Invalid header
		}

		return parsed > config.Get().Uploads.MaxSizeBytes
	}

	return false // We can only assume
}

func UploadMedia(contents io.ReadCloser, contentType string, filename string, userId string, origin string, isPublic bool, ctx context.Context, log *logrus.Entry) (*types.Media, error) {
	defer contents.Close()

	var data io.Reader
	if config.Get().Uploads.MaxSizeBytes > 0 {
		data = io.LimitReader(contents, config.Get().Uploads.MaxSizeBytes)
	} else {
		data = contents
	}

	mediaId, err := util.GenerateRandomString(64)
	if err != nil {
		return nil, err
	}

	var contentToken *string
	if !isPublic {
		generatedToken, err := util.GenerateRandomString(128)
		if err != nil {
			return nil, err
		}

		contentToken = &generatedToken
	}

	stored, err := StoreDirect(data, contentType, filename, userId, origin, mediaId, contentToken, ctx, log)
	if err != nil {
		return nil, err
	}

	return stored, nil
}

func StoreDirect(contents io.Reader, contentType string, filename string, userId string, origin string, mediaId string, contentToken *string, ctx context.Context, log *logrus.Entry) (*types.Media, error) {
	fileLocation, err := storage.PersistFile(contents, ctx, log)
	if err != nil {
		return nil, err
	}

	fileMime, err := util.GetMimeType(fileLocation)
	if err != nil {
		log.Error("Error while checking content type of file: ", err.Error())
		os.Remove(fileLocation) // delete temp file
		return nil, err
	}

	for _, allowedType := range config.Get().Uploads.AllowedTypes {
		if !glob.Glob(allowedType, fileMime) {
			log.Warn("Content type " + fileMime + " (reported as " + contentType + ") is not allowed to be uploaded")

			os.Remove(fileLocation) // delete temp file
			return nil, common.ErrMediaNotAllowed
		}
	}

	hash, err := storage.GetFileHash(fileLocation)
	if err != nil {
		os.Remove(fileLocation) // delete temp file
		return nil, err
	}

	db := storage.GetDatabase().GetMediaStore(ctx, log)
	records, err := db.GetByHash(hash)
	if err != nil {
		os.Remove(fileLocation) // delete temp file
		return nil, err
	}

	if len(records) > 0 {
		log.Info("Duplicate media for hash ", hash)

		// We'll use the location from the first record
		media := records[0]
		media.Origin = origin
		media.MediaId = mediaId
		media.UserId = userId
		media.UploadName = filename
		media.ContentType = contentType
		media.CreationTs = util.NowMillis()
		media.ContentToken = contentToken

		err = db.Insert(media)
		if err != nil {
			os.Remove(fileLocation) // delete temp file
			return nil, err
		}

		// If the media's file exists, we'll delete the temp file
		// If the media's file doesn't exist, we'll move the temp file to where the media expects it to be
		exists, err := util.FileExists(media.Location)
		if err != nil || !exists {
			// We'll assume an error means it doesn't exist
			os.Rename(fileLocation, media.Location)
		} else {
			os.Remove(fileLocation)
		}

		return media, nil
	}

	// The media doesn't already exist - save it as new

	fileSize, err := util.FileSize(fileLocation)
	if err != nil {
		os.Remove(fileLocation) // delete temp file
		return nil, err
	}

	log.Info("Persisting new media record")

	media := &types.Media{
		Origin:       origin,
		MediaId:      mediaId,
		UploadName:   filename,
		ContentType:  contentType,
		UserId:       userId,
		Sha256Hash:   hash,
		SizeBytes:    fileSize,
		Location:     fileLocation,
		CreationTs:   util.NowMillis(),
		ContentToken: contentToken,
	}

	err = db.Insert(media)
	if err != nil {
		os.Remove(fileLocation) // delete temp file
		return nil, err
	}

	return media, nil
}
