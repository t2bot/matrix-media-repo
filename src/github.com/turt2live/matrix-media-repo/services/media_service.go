package services

import (
	"database/sql"
	"io"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services/handlers"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaService struct {
	store *stores.MediaStore
	i     rcontext.RequestInfo
}

func CreateMediaService(i rcontext.RequestInfo) (*MediaService) {
	return &MediaService{i.Db.GetMediaStore(i.Context, i.Log), i}
}

func (s *MediaService) GetMedia(server string, mediaId string) (types.Media, error) {
	s.i.Log.Info("Looking up media")
	media, err := s.store.Get(server, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			if util.IsServerOurs(server) {
				s.i.Log.Warn("Media not found")
				return media, util.ErrMediaNotFound
			}
		}

		return s.downloadRemoteMedia(server, mediaId)
	}

	exists, err := util.FileExists(media.Location)
	if !exists || err != nil {
		if util.IsServerOurs(server) {
			s.i.Log.Error("Media not found in file store when we expected it to")
			return media, util.ErrMediaNotFound
		} else {
			s.i.Log.Warn("Media appears to have been deleted - redownloading")
			return s.downloadRemoteMedia(server, mediaId)
		}
	}

	return media, nil
}

func (s *MediaService) downloadRemoteMedia(server string, mediaId string) (types.Media, error) {
	s.i.Log.Info("Attempting to download remote media")
	downloader := &handlers.RemoteMediaDownloader{
		Info:       s.i,
		MediaStore: *s.store,
	}

	downloaded, err := downloader.Download(server, mediaId)
	if err != nil {
		return types.Media{}, err
	}

	defer downloaded.Contents.Close()
	return s.StoreMedia(downloaded.Contents, downloaded.ContentType, downloaded.DesiredFilename, "", server, mediaId)
}

func (s *MediaService) IsTooLarge(contentLength int64, contentLengthHeader string) (bool) {
	if config.Get().Uploads.MaxSizeBytes <= 0 {
		return false
	}
	if contentLength >= 0 {
		return contentLength > config.Get().Uploads.MaxSizeBytes
	}
	if contentLengthHeader != "" {
		parsed, err := strconv.ParseInt(contentLengthHeader, 10, 64)
		if err != nil {
			s.i.Log.Warn("Invalid content length header given; assuming too large. Value received: " + contentLengthHeader)
			return true // Invalid header
		}

		return parsed > config.Get().Uploads.MaxSizeBytes
	}

	return false // We can only assume
}

func (s *MediaService) UploadMedia(contents io.ReadCloser, contentType string, filename string, userId string, host string) (types.Media, error) {
	defer contents.Close()
	var data io.Reader
	if config.Get().Uploads.MaxSizeBytes > 0 {
		data = io.LimitReader(contents, config.Get().Uploads.MaxSizeBytes)
	} else {
		data = contents
	}

	return s.StoreMedia(data, contentType, filename, userId, host, "")
}

func (s *MediaService) StoreMedia(contents io.Reader, contentType string, filename string, userId string, host string, mediaId string) (types.Media, error) {
	isGeneratedId := false
	if mediaId == "" {
		mediaId = generateMediaId()
		isGeneratedId = true
	}
	log := s.i.Log.WithFields(logrus.Fields{
		"mediaService_mediaId":            mediaId,
		"mediaService_host":               host,
		"mediaService_mediaIdIsGenerated": isGeneratedId,
	})

	// Store the file in a temporary location
	fileLocation, err := storage.PersistFile(contents, s.i.Context, &s.i.Db)
	if err != nil {
		return types.Media{}, err
	}

	hash, err := storage.GetFileHash(fileLocation)
	if err != nil {
		defer os.Remove(fileLocation) // attempt cleanup
		return types.Media{}, err
	}

	records, err := s.store.GetByHash(hash)
	if err != nil {
		defer os.Remove(fileLocation) // attempt cleanup
		return types.Media{}, err
	}

	// If there's at least one record, then we have a duplicate hash - try and process it
	if len(records) > 0 {
		// See if we one of the duplicate records is a match for the host and media ID. We'll otherwise use
		// the last duplicate (should only be 1 anyways) as our starting point for a new record.
		var media types.Media
		for i := 0; i < len(records); i++ {
			media = records[i]

			if media.Origin == host && (media.MediaId == mediaId || isGeneratedId) {
				if media.ContentType != contentType || media.UserId != userId || media.UploadName != filename {
					// The unique constraint in the database prevents us from storing a duplicate, and we can't generate a new
					// media ID because then we'd be discarding the caller's media ID. In practice, this particular branch would
					// only be executed if a file over federation got changed and we, for some reason, re-downloaded it.
					log.Warn("Match found for media based on host and media ID. Filename, content type, or user ID may not match. Returning unaltered media record")
				} else {
					log.Info("Match found for media based on host and media ID. Returning unaltered media record.")
				}

				overwriteExistingOrDeleteTempFile(fileLocation, media)
				return media, nil
			}

			// The last media object will be used to create a new pointer (normally there should only be one anyways)
		}

		log.Info("Duplicate media hash found, generating a new record using the existing file")

		media.Origin = host
		media.UserId = userId
		media.MediaId = mediaId
		media.UploadName = filename
		media.ContentType = contentType
		media.CreationTs = util.NowMillis()

		err = s.store.Insert(&media)
		if err != nil {
			return types.Media{}, err
		}

		overwriteExistingOrDeleteTempFile(fileLocation, media)
		return media, nil
	}

	// No duplicate hash, so we have to create a new record

	fileSize, err := util.FileSize(fileLocation)
	if err != nil {
		defer os.Remove(fileLocation) // attempt cleanup
		return types.Media{}, err
	}

	log.Info("Persisting unique media record")

	media := &types.Media{
		Origin:      host,
		MediaId:     mediaId,
		UploadName:  filename,
		ContentType: contentType,
		UserId:      userId,
		Sha256Hash:  hash,
		SizeBytes:   fileSize,
		Location:    fileLocation,
		CreationTs:  util.NowMillis(),
	}

	err = s.store.Insert(media)
	if err != nil {
		defer os.Remove(fileLocation) // attempt cleanup
		return types.Media{}, err
	}

	return *media, nil
}

func generateMediaId() string {
	str, err := util.GenerateRandomString(64)
	if err != nil {
		panic(err)
	}

	return str
}

func overwriteExistingOrDeleteTempFile(tempFileLocation string, media types.Media) {
	// If the media's file exists, we'll delete the temp file
	// If the media's file doesn't exist, we'll move the temp file to where the media expects it to be
	exists, err := util.FileExists(media.Location)
	if err != nil || !exists {
		// We'll assume an error means it doesn't exist
		os.Rename(tempFileLocation, media.Location)
	} else {
		os.Remove(tempFileLocation)
	}
}
