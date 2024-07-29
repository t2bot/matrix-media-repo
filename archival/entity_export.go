package archival

import (
	"errors"
	"time"

	"github.com/t2bot/matrix-media-repo/archival/v2archive"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_download"
	"github.com/t2bot/matrix-media-repo/util"
)

func ExportEntityData(ctx rcontext.RequestContext, exportId string, entityId string, exportS3Urls bool, writeFn v2archive.PartPersister) error {
	archiver, err := v2archive.NewWriter(ctx, exportId, entityId, ctx.Config.Archiving.TargetBytesPerPart, writeFn)
	if err != nil {
		return err
	}
	defer func(archiver *v2archive.ArchiveWriter) {
		ctx.Log.Debug("Finishing export archive")
		_ = archiver.Finish()
	}(archiver)

	db := database.GetInstance().Media.Prepare(ctx)

	ctx.Log.Debug("Retrieving media records for entity")
	var records []*database.DbMedia
	if entityId[0] == '@' {
		records, err = db.GetByUserId(entityId)
	} else {
		records, err = db.GetByOrigin(entityId)
	}
	if err != nil {
		return err
	}

	ctx.Log.Infof("Exporting %d media records", len(records))
	for _, media := range records {
		mxc := util.MxcUri(media.Origin, media.MediaId)
		ctx.Log.Debugf("Downloading %s", mxc)
		_, s, err := pipeline_download.Execute(ctx, media.Origin, media.MediaId, pipeline_download.DownloadOpts{
			FetchRemoteIfNeeded: false,
			BlockForReadUntil:   10 * time.Minute,
			RecordOnly:          false,
			AuthProvided:        true, // it's for an export, so assume authentication
		})
		if errors.Is(err, common.ErrMediaQuarantined) {
			ctx.Log.Warnf("%s is quarantined and will not be included in the export", mxc)
			continue
		} else if errors.Is(err, common.ErrMediaNotYetUploaded) {
			ctx.Log.Debug("Media not uploaded yet - skipping")
			continue
		} else if err != nil {
			return err
		}
		s3url := ""
		if exportS3Urls {
			if dsConf, ok := datastores.Get(ctx, media.DatastoreId); !ok {
				// "should never happen" because we downloaded the file, in theory
				ctx.Log.Warnf("Cannot populate S3 URL for %s because datastore for media could not be found", mxc)
			} else {
				s3url, err = datastores.GetS3Url(dsConf, media.Location)
				if err != nil {
					ctx.Log.Warnf("Cannot populate S3 URL for %s because there was an error getting S3 information: %s", mxc, err)
				}
			}
		}
		exportedHash, err := archiver.AppendMedia(s, v2archive.MediaInfo{
			Origin:      media.Origin,
			MediaId:     media.MediaId,
			FileName:    media.UploadName,
			ContentType: media.ContentType,
			CreationTs:  media.CreationTs,
			S3Url:       s3url,
			UserId:      media.UserId,
		})
		if err != nil {
			return err
		}
		if exportedHash != media.Sha256Hash {
			ctx.Log.Warnf("%s should have had hash %s but it had %s when placed in the archive", mxc, media.Sha256Hash, exportedHash)
		}
	}

	return nil
}
