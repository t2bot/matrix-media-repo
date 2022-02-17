package storage

import (
	"database/sql"
	"github.com/turt2live/matrix-media-repo/util/ids"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
)

func GetOrCreateDatastoreOfType(ctx rcontext.RequestContext, dsType string, dsUri string) (*types.Datastore, error) {
	mediaService := GetDatabase().GetMediaStore(ctx)
	datastore, err := mediaService.GetDatastoreByUri(dsUri)
	if err != nil && err == sql.ErrNoRows {
		id, err2 := ids.NewUniqueId()
		if err2 != nil {
			ctx.Log.Error("Error generating datastore ID for URI ", dsUri, ": ", err)
			return nil, err2
		}
		datastore = &types.Datastore{
			DatastoreId: id,
			Type:        dsType,
			Uri:         dsUri,
		}
		err2 = mediaService.InsertDatastore(datastore)
		if err2 != nil {
			ctx.Log.Error("Error creating datastore for URI ", dsUri, ": ", err)
			return nil, err2
		}
	}
	return datastore, nil
}

func getOrCreateDatastoreWithMediaService(mediaService *stores.MediaStore, basePath string) (*types.Datastore, error) {
	datastore, err := mediaService.GetDatastoreByUri(basePath)
	if err != nil && err == sql.ErrNoRows {
		id, err2 := ids.NewUniqueId()
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
