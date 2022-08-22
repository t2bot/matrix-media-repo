package upload_controller

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/ryanuber/go-glob"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/internal_cache"
	"github.com/turt2live/matrix-media-repo/plugins"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
)

const NoApplicableUploadUser = ""

var recentMediaIds = cache.New(30*time.Second, 60*time.Second)

type AlreadyUploadedFile struct {
	DS         *datastore.DatastoreRef
	ObjectInfo *types.ObjectInfo
}

func IsRequestTooLargeForUser(contentLength int64, contentLengthHeader string, ctx rcontext.RequestContext, userId string) bool {
	if !ctx.Config.Uploads.MaxBytesPerUser.Enabled {
		return false // per user max not enabled
	}

	for _, q := range ctx.Config.Uploads.MaxBytesPerUser.UserMaxBytes {
		if glob.Glob(q.Glob, userId) {
			if q.MaxBytes == 0 {
				return false // no max for user
			}
			return contentLength > q.MaxBytes
		}
	}

	return false // no rules == no max
}

func IsRequestTooLarge(contentLength int64, contentLengthHeader string, ctx rcontext.RequestContext) bool {
	if ctx.Config.Uploads.MaxSizeBytes <= 0 {
		return false
	}
	if contentLength >= 0 {
		return contentLength > ctx.Config.Uploads.MaxSizeBytes
	}
	if contentLengthHeader != "" {
		parsed, err := strconv.ParseInt(contentLengthHeader, 10, 64)
		if err != nil {
			ctx.Log.Warn("Invalid content length header given; assuming too large. Value received: " + contentLengthHeader)
			sentry.CaptureException(err)
			return true // Invalid header
		}

		return parsed > ctx.Config.Uploads.MaxSizeBytes
	}

	return false // We can only assume
}

func IsRequestTooSmall(contentLength int64, contentLengthHeader string, ctx rcontext.RequestContext) bool {
	if ctx.Config.Uploads.MinSizeBytes <= 0 {
		return false
	}
	if contentLength >= 0 {
		return contentLength < ctx.Config.Uploads.MinSizeBytes
	}
	if contentLengthHeader != "" {
		parsed, err := strconv.ParseInt(contentLengthHeader, 10, 64)
		if err != nil {
			ctx.Log.Warn("Invalid content length header given; assuming too small. Value received: " + contentLengthHeader)
			sentry.CaptureException(err)
			return true // Invalid header
		}

		return parsed < ctx.Config.Uploads.MinSizeBytes
	}

	return false // We can only assume
}

func EstimateContentLength(contentLength int64, contentLengthHeader string) int64 {
	if contentLength >= 0 {
		return contentLength
	}
	if contentLengthHeader != "" {
		parsed, err := strconv.ParseInt(contentLengthHeader, 10, 64)
		if err != nil {
			logrus.Warn("Invalid content length header given. Value received: " + contentLengthHeader)
			sentry.CaptureException(err)
			return -1 // unknown
		}

		return parsed
	}

	return -1 // unknown
}

func UploadMedia(contents io.ReadCloser, contentLength int64, contentType string, filename string, userId string, origin string, ctx rcontext.RequestContext) (*types.Media, error) {
	defer cleanup.DumpAndCloseStream(contents)

	var data io.ReadCloser
	if ctx.Config.Uploads.MaxSizeBytes > 0 {
		data = ioutil.NopCloser(io.LimitReader(contents, ctx.Config.Uploads.MaxSizeBytes))
	} else {
		data = contents
	}

	dataBytes, err := ioutil.ReadAll(data)
	if err != nil {
		return nil, err
	}

	metadataDb := storage.GetDatabase().GetMetadataStore(ctx)

	mediaTaken := true
	var mediaId string
	attempts := 0
	for mediaTaken {
		attempts += 1
		if attempts > 10 {
			return nil, errors.New("failed to generate a media ID after 10 rounds")
		}

		mediaId, err = util.GenerateRandomString(64)
		if err != nil {
			return nil, err
		}
		mediaId, err = util.GetSha1OfString(mediaId + strconv.FormatInt(util.NowMillis(), 10))
		if err != nil {
			return nil, err
		}

		// Because we use the current time in the media ID, we don't need to worry about
		// collisions from the database.
		if _, present := recentMediaIds.Get(mediaId); present {
			mediaTaken = true
			continue
		}

		mediaTaken, err = metadataDb.IsReserved(origin, mediaId)
		if err != nil {
			return nil, err
		}
	}

	_ = recentMediaIds.Add(mediaId, true, cache.DefaultExpiration)

	var existingFile *AlreadyUploadedFile = nil
	ds, err := datastore.PickDatastore(common.KindLocalMedia, ctx)
	if err != nil {
		return nil, err
	}
	if ds.Type == "ipfs" {
		// Do the upload now so we can pick the media ID to point to IPFS
		info, err := ds.UploadFile(util_byte_seeker.NewByteSeeker(dataBytes), contentLength, ctx)
		if err != nil {
			return nil, err
		}
		existingFile = &AlreadyUploadedFile{
			DS:         ds,
			ObjectInfo: info,
		}
		mediaId = fmt.Sprintf("ipfs:%s", info.Location[len("ipfs/"):])
	}

	m, err := StoreDirect(existingFile, util_byte_seeker.NewByteSeeker(dataBytes), contentLength, contentType, filename, userId, origin, mediaId, common.KindLocalMedia, ctx, true)
	if err != nil {
		return m, err
	}
	if m != nil {
		err = internal_cache.Get().UploadMedia(m.Sha256Hash, util_byte_seeker.NewByteSeeker(dataBytes), ctx)
		if err != nil {
			ctx.Log.Warn("Unexpected error trying to cache media: " + err.Error())
		}
	}
	return m, err
}

func trackUploadAsLastAccess(ctx rcontext.RequestContext, media *types.Media) {
	err := storage.GetDatabase().GetMetadataStore(ctx).UpsertLastAccess(media.Sha256Hash, util.NowMillis())
	if err != nil {
		logrus.Warn("Failed to upsert the last access time: ", err)
	}
}

func checkSpam(contents []byte, filename string, contentType string, userId string, origin string, mediaId string) error {
	spam, err := plugins.CheckForSpam(contents, filename, contentType, userId, origin, mediaId)
	if err != nil {
		logrus.Warn("Error checking spam - assuming not spam: " + err.Error())
		sentry.CaptureException(err)
		return nil
	}
	if spam {
		return common.ErrMediaQuarantined
	}
	return nil
}

func StoreDirect(f *AlreadyUploadedFile, contents io.ReadCloser, expectedSize int64, contentType string, filename string, userId string, origin string, mediaId string, kind string, ctx rcontext.RequestContext, filterUserDuplicates bool) (*types.Media, error) {
	var err error
	var ds *datastore.DatastoreRef
	var info *types.ObjectInfo
	var contentBytes []byte
	if f == nil {
		dsPicked, err := datastore.PickDatastore(kind, ctx)
		if err != nil {
			return nil, err
		}
		ds = dsPicked

		contentBytes, err = ioutil.ReadAll(contents)
		if err != nil {
			return nil, err
		}

		fInfo, err := ds.UploadFile(util.BytesToStream(contentBytes), expectedSize, ctx)
		if err != nil {
			return nil, err
		}
		info = fInfo
	} else {
		ds = f.DS
		info = f.ObjectInfo

		// download the contents for antispam
		contents, err = ds.DownloadFile(info.Location)
		if err != nil {
			return nil, err
		}
		contentBytes, err = ioutil.ReadAll(contents)
		if err != nil {
			return nil, err
		}
	}

	db := storage.GetDatabase().GetMediaStore(ctx)
	records, err := db.GetByHash(info.Sha256Hash)
	if err != nil {
		err2 := ds.DeleteObject(info.Location) // delete temp object
		if err2 != nil {
			ctx.Log.Warn("Error deleting temporary upload", err2)
			sentry.CaptureException(err2)
		}
		return nil, err
	}

	if len(records) > 0 {
		ctx.Log.Info("Duplicate media for hash ", info.Sha256Hash)

		// If the user is a real user (ie: actually uploaded media), then we'll see if there's
		// an exact duplicate that we can return. Otherwise we'll just pick the first record and
		// clone that.
		if filterUserDuplicates && userId != NoApplicableUploadUser {
			for _, record := range records {
				if record.Quarantined {
					ctx.Log.Warn("User attempted to upload quarantined content - rejecting")
					return nil, common.ErrMediaQuarantined
				}
				if record.UserId == userId && record.Origin == origin && record.ContentType == contentType {
					ctx.Log.Info("User has already uploaded this media before - returning unaltered media record")

					err2 := ds.DeleteObject(info.Location) // delete temp object
					if err2 != nil {
						ctx.Log.Warn("Error deleting temporary upload", err2)
						sentry.CaptureException(err2)
					}

					trackUploadAsLastAccess(ctx, record)
					return record, nil
				}
			}
		}

		err = checkSpam(contentBytes, filename, contentType, userId, origin, mediaId)
		if err != nil {
			err2 := ds.DeleteObject(info.Location) // delete temp object
			if err2 != nil {
				ctx.Log.Warn("Error deleting temporary upload", err2)
				sentry.CaptureException(err2)
			}
			return nil, err
		}

		// We'll use the location from the first record
		record := records[0]
		if record.Quarantined {
			err2 := ds.DeleteObject(info.Location) // delete temp object
			if err2 != nil {
				ctx.Log.Warn("Error deleting temporary upload", err2)
				sentry.CaptureException(err2)
			}
			ctx.Log.Warn("User attempted to upload quarantined content - rejecting")
			return nil, common.ErrMediaQuarantined
		}

		// Double check that we're not about to try and store a record we know about
		for _, knownRecord := range records {
			if knownRecord.Origin == origin && knownRecord.MediaId == mediaId {
				ctx.Log.Info("Duplicate media record found - returning unaltered record")
				err2 := ds.DeleteObject(info.Location) // delete temp object
				if err2 != nil {
					ctx.Log.Warn("Error deleting temporary upload", err2)
					sentry.CaptureException(err2)
				}
				trackUploadAsLastAccess(ctx, knownRecord)
				return knownRecord, nil
			}
		}

		media := record
		media.Origin = origin
		media.MediaId = mediaId
		media.UserId = userId
		media.UploadName = filename
		media.ContentType = contentType
		media.CreationTs = util.NowMillis()

		err = db.Insert(media)
		if err != nil {
			err2 := ds.DeleteObject(info.Location) // delete temp object
			if err2 != nil {
				ctx.Log.Warn("Error deleting temporary upload", err2)
				sentry.CaptureException(err2)
			}
			return nil, err
		}

		// If the media's file exists, we'll delete the temp file
		// If the media's file doesn't exist, we'll move the temp file to where the media expects it to be
		if media.DatastoreId != ds.DatastoreId && media.Location != info.Location {
			ds2, err := datastore.LocateDatastore(ctx, media.DatastoreId)
			if err != nil {
				err2 := ds.DeleteObject(info.Location) // delete temp object
				if err2 != nil {
					ctx.Log.Warn("Error deleting temporary upload", err2)
					sentry.CaptureException(err2)
				}
				return nil, err
			}
			if !ds2.ObjectExists(media.Location) {
				stream, err := ds.DownloadFile(info.Location)
				if err != nil {
					return nil, err
				}

				err2 := ds2.OverwriteObject(media.Location, stream, ctx)
				if err2 != nil {
					ctx.Log.Warn("Error overwriting object", err2)
					sentry.CaptureException(err2)
				}
				err2 = ds.DeleteObject(info.Location) // delete temp object
				if err2 != nil {
					ctx.Log.Warn("Error deleting temporary upload", err2)
					sentry.CaptureException(err2)
				}
			} else {
				err2 := ds.DeleteObject(info.Location) // delete temp object
				if err2 != nil {
					ctx.Log.Warn("Error deleting temporary upload", err2)
					sentry.CaptureException(err2)
				}
			}
		}

		trackUploadAsLastAccess(ctx, media)
		return media, nil
	}

	// The media doesn't already exist - save it as new

	if info.SizeBytes <= 0 {
		err2 := ds.DeleteObject(info.Location) // delete temp object
		if err2 != nil {
			ctx.Log.Warn("Error deleting temporary upload", err2)
			sentry.CaptureException(err2)
		}
		return nil, errors.New("file has no contents")
	}

	err = checkSpam(contentBytes, filename, contentType, userId, origin, mediaId)
	if err != nil {
		err2 := ds.DeleteObject(info.Location) // delete temp object
		if err2 != nil {
			ctx.Log.Warn("Error deleting temporary upload", err2)
			sentry.CaptureException(err2)
		}
		return nil, err
	}

	ctx.Log.Info("Persisting new media record")

	media := &types.Media{
		Origin:      origin,
		MediaId:     mediaId,
		UploadName:  filename,
		ContentType: contentType,
		UserId:      userId,
		Sha256Hash:  info.Sha256Hash,
		SizeBytes:   info.SizeBytes,
		DatastoreId: ds.DatastoreId,
		Location:    info.Location,
		CreationTs:  util.NowMillis(),
	}

	err = db.Insert(media)
	if err != nil {
		err2 := ds.DeleteObject(info.Location) // delete temp object
		if err2 != nil {
			ctx.Log.Warn("Error deleting temporary upload", err2)
			sentry.CaptureException(err2)
		}
		return nil, err
	}

	trackUploadAsLastAccess(ctx, media)
	return media, nil
}
