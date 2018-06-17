package storage

import (
	"context"

	"github.com/sirupsen/logrus"
)

func populateThumbnailHashes(db *Database) (error) {
	svc := db.GetThumbnailStore(context.TODO(), &logrus.Entry{})

	thumbs, err := svc.GetAllWithoutHash()
	if err != nil {
		logrus.Error("Failed to get thumbnails that don't have hashes: ", err)
		return err
	}

	for _, thumb := range thumbs {
		hash, err := GetFileHash(thumb.Location)
		if err != nil {
			logrus.Error("Failed to generate hash for location '", thumb.Location, "': ", err)
			return err
		}

		thumb.Sha256Hash = hash
		err = svc.UpdateHash(thumb)
		if err != nil {
			logrus.Error("Failed to update hash for '", thumb.Location, "': ", err)
			return err
		}

		logrus.Info("Updated hash for thumbnail at '", thumb.Location, "' as ", hash)
	}

	return nil
}
