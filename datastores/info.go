package datastores

import (
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
)

type SizeEstimate struct {
	ThumbnailsAffected      int64 `json:"thumbnails_affected"`
	ThumbnailHashesAffected int64 `json:"thumbnail_hashes_affected"`
	ThumbnailBytes          int64 `json:"thumbnail_bytes"`

	MediaAffected       int64 `json:"media_affected"`
	MediaHashesAffected int64 `json:"media_hashes_affected"`
	MediaBytes          int64 `json:"media_bytes"`

	TotalHashesAffected int64 `json:"total_hashes_affected"`
	TotalBytes          int64 `json:"total_bytes"`
}

func GetUri(ds config.DatastoreConfig) (string, error) {
	if ds.Type == "s3" {
		s3c, err := getS3(ds)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("s3://%s/%s", s3c.client.EndpointURL().Hostname(), s3c.bucket), nil
	} else if ds.Type == "file" {
		return ds.Options["path"], nil
	} else {
		return "", errors.New("unknown datastore type - contact developer")
	}
}

func SizeOfDsIdWithAge(ctx rcontext.RequestContext, dsId string, beforeTs int64) (*SizeEstimate, error) {
	db := database.GetInstance().MetadataView.Prepare(ctx)
	media, err := db.GetMediaForDatastoreByLastAccess(dsId, beforeTs)
	if err != nil {
		return nil, err
	}
	thumbs, err := db.GetThumbnailsForDatastoreByLastAccess(dsId, beforeTs)
	if err != nil {
		return nil, err
	}

	estimate := &SizeEstimate{}
	seenHashes := make(map[string]bool)
	seenMediaHashes := make(map[string]bool)
	seenThumbnailHashes := make(map[string]bool)

	for _, record := range media {
		estimate.MediaAffected++

		if _, found := seenHashes[record.Sha256Hash]; !found {
			estimate.TotalBytes += record.SizeBytes
			estimate.TotalHashesAffected++
		}
		if _, found := seenMediaHashes[record.Sha256Hash]; !found {
			estimate.MediaBytes += record.SizeBytes
			estimate.MediaHashesAffected++
		}

		seenHashes[record.Sha256Hash] = true
		seenMediaHashes[record.Sha256Hash] = true
	}
	for _, record := range thumbs {
		estimate.ThumbnailsAffected++

		if _, found := seenHashes[record.Sha256Hash]; !found {
			estimate.TotalBytes += record.SizeBytes
			estimate.TotalHashesAffected++
		}
		if _, found := seenThumbnailHashes[record.Sha256Hash]; !found {
			estimate.ThumbnailBytes += record.SizeBytes
			estimate.ThumbnailHashesAffected++
		}

		seenHashes[record.Sha256Hash] = true
		seenThumbnailHashes[record.Sha256Hash] = true
	}

	return estimate, nil
}
