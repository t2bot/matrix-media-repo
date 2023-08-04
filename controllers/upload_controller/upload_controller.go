package upload_controller

import (
	"bytes"
	"errors"
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/plugins"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

const NoApplicableUploadUser = ""

type AlreadyUploadedFile struct {
	DS         *datastore.DatastoreRef
	ObjectInfo *types.ObjectInfo
}

func trackUploadAsLastAccess(ctx rcontext.RequestContext, media *types.Media) {
	err := storage.GetDatabase().GetMetadataStore(ctx).UpsertLastAccess(media.Sha256Hash, util.NowMillis())
	if err != nil {
		logrus.Warn("Failed to upsert the last access time: ", err)
	}
}

func checkSpam(contents []byte, filename string, contentType string, userId string, origin string, mediaId string) error {
	spam, err := plugins.CheckForSpam(bytes.NewBuffer(contents), filename, contentType, userId, origin, mediaId)
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

func StoreDirect(f *AlreadyUploadedFile, contents io.ReadCloser, expectedSize int64, contentType string, filename string, userId string, origin string, mediaId string, kind common.Kind, ctx rcontext.RequestContext, filterUserDuplicates bool) (*types.Media, error) {
	var err error
	var ds *datastore.DatastoreRef
	var info *types.ObjectInfo
	var contentBytes []byte
	if f == nil {
		dsPicked, err := datastore.PickDatastore(string(kind), ctx)
		if err != nil {
			return nil, err
		}
		ds = dsPicked

		contentBytes, err = io.ReadAll(contents)
		if err != nil {
			return nil, err
		}

		fInfo, err := ds.UploadFile(stream_util.BytesToStream(contentBytes), expectedSize, ctx)
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
		contentBytes, err = io.ReadAll(contents)
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
