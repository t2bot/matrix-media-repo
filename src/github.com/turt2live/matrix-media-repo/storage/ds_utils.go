package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func GetOrCreateDatastore(ctx context.Context, log *logrus.Entry, basePath string) (*types.Datastore, error) {
	mediaService := GetDatabase().GetMediaStore(ctx, log)
	return getOrCreateDatastoreWithMediaService(mediaService, basePath)
}

func getOrCreateDatastoreWithMediaService(mediaService *stores.MediaStore, basePath string) (*types.Datastore, error) {
	datastore, err := mediaService.GetDatastoreByUri(basePath)
	if err != nil && err == sql.ErrNoRows {
		id, err2 := util.GenerateRandomString(32)
		if err2 != nil {
			logrus.Error("Error generating datastore ID for base path ", basePath, ": ", err)
			return nil, err2
		}
		datastore = &types.Datastore{
			DatastoreId: id,
			Type:        "file",
			Uri:         basePath,
		}
		err2 = mediaService.InsertDatastore(datastore)
		if err2 != nil {
			logrus.Error("Error creating datastore for base path ", basePath, ": ", err)
			return nil, err2
		}
	} else if err != nil {
		logrus.Error("Error getting datastore for base path ", basePath, ": ", err)
		return nil, err
	}

	return datastore, nil
}

func ResolveMediaLocation(ctx context.Context, log *logrus.Entry, datastoreId string, location string) (string, error) {
	svc := GetDatabase().GetMediaStore(ctx, log)
	ds, err := svc.GetDatastore(datastoreId)
	if err != nil {
		return "", err
	}

	if ds.Type != "file" {
		return "", errors.New("unrecognized datastore type: " + ds.Type)
	}

	return ds.ResolveFilePath(location), nil
}
