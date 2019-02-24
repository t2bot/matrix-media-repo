package storage

import (
	"context"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_file"
	"github.com/turt2live/matrix-media-repo/util"
)

func populateThumbnailHashes(db *Database) (error) {
	svc := db.GetThumbnailStore(context.TODO(), &logrus.Entry{})
	mediaSvc := db.GetMediaStore(context.TODO(), &logrus.Entry{})

	thumbs, err := svc.GetAllWithoutHash()
	if err != nil {
		logrus.Error("Failed to get thumbnails that don't have hashes: ", err)
		return err
	}

	for _, thumb := range thumbs {
		datastore, err := mediaSvc.GetDatastore(thumb.DatastoreId)
		if err != nil {
			logrus.Error("Error getting datastore for thumbnail ", thumb.Origin, " ", thumb.MediaId, ": ", err)
			continue
		}
		if datastore.Type != "file" {
			logrus.Error("Unrecognized datastore type for thumbnail ", thumb.Origin, " ", thumb.MediaId)
			continue
		}
		location := datastore.ResolveFilePath(thumb.Location)

		hash, err := ds_file.GetFileHash(location)
		if err != nil {
			logrus.Error("Failed to generate hash for location '", location, "': ", err)
			return err
		}

		thumb.Sha256Hash = hash
		err = svc.UpdateHash(thumb)
		if err != nil {
			logrus.Error("Failed to update hash for '", location, "': ", err)
			return err
		}

		logrus.Info("Updated hash for thumbnail at '", location, "' as ", hash)
	}

	return nil
}

func populateDatastores(db *Database) (error) {
	logrus.Info("Starting to populate datastores...")

	thumbService := db.GetThumbnailStore(context.TODO(), &logrus.Entry{})
	mediaService := db.GetMediaStore(context.TODO(), &logrus.Entry{})

	logrus.Info("Fetching thumbnails...")
	thumbs, err := thumbService.GetAllWithoutDatastore()
	if err != nil {
		logrus.Error("Failed to get thumbnails that don't have a datastore: ", err)
		return err
	}

	for _, thumb := range thumbs {
		basePath := path.Clean(path.Join(thumb.Location, "..", "..", ".."))
		datastore, err := getOrCreateDatastoreWithMediaService(mediaService, basePath)
		if err != nil {
			logrus.Error("Error getting datastore for thumbnail path ", basePath, ": ", err)
			continue
		}

		thumb.DatastoreId = datastore.DatastoreId
		thumb.Location = util.GetLastSegmentsOfPath(thumb.Location, 3)

		err = thumbService.UpdateDatastoreAndLocation(thumb)
		if err != nil {
			logrus.Error("Failed to update datastore for thumbnail ", thumb.Origin, " ", thumb.MediaId, ": ", err)
			continue
		}

		logrus.Info("Updated datastore for thumbnail ", thumb.Origin, " ", thumb.MediaId)
	}

	logrus.Info("Fetching media...")
	mediaRecords, err := mediaService.GetAllWithoutDatastore()
	if err != nil {
		logrus.Error("Failed to get media that doesn't have a datastore: ", err)
		return err
	}

	for _, media := range mediaRecords {
		basePath := path.Clean(path.Join(media.Location, "..", "..", ".."))
		datastore, err := getOrCreateDatastoreWithMediaService(mediaService, basePath)
		if err != nil {
			logrus.Error("Error getting datastore for media path ", basePath, ": ", err)
			continue
		}

		media.DatastoreId = datastore.DatastoreId
		media.Location = util.GetLastSegmentsOfPath(media.Location, 3)

		err = mediaService.UpdateDatastoreAndLocation(media)
		if err != nil {
			logrus.Error("Failed to update datastore for media ", media.Origin, " ", media.MediaId, ": ", err)
			continue
		}

		logrus.Info("Updated datastore for media ", media.Origin, " ", media.MediaId)
	}

	return nil
}
