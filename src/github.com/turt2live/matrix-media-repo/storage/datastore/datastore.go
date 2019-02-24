package datastore

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
)

// TODO: Upload to DS
// TODO: Download (get stream) from DS

func GetUriForDatastore(dsConf config.DatastoreConfig) string {
	if dsConf.Type == "file" {
		path, pathFound := dsConf.Options["path"]
		if !pathFound {
			logrus.Fatal("Missing 'path' on file datastore")
		}
		return path
	} else if dsConf.Type == "s3" {
		endpoint, epFound := dsConf.Options["endpoint"]
		bucket, bucketFound := dsConf.Options["bucketName"]
		if !epFound || !bucketFound {
			logrus.Fatal("Missing 'endpoint' or 'bucketName' on s3 datastore")
		}
		return fmt.Sprintf("s3://%s/%s", endpoint, bucket)
	} else {
		logrus.Fatal("Unknown datastore type: ", dsConf.Type)
	}

	return ""
}

func GetDatastoreConfig(ds *types.Datastore) (config.DatastoreConfig, error) {
	for _, dsConf := range config.Get().DataStores {
		if dsConf.Type == ds.Type && GetUriForDatastore(dsConf) == ds.Uri {
			return dsConf, nil
		}
	}

	return config.DatastoreConfig{}, errors.New("datastore not found")
}

func PickDatastore(ctx context.Context, log *logrus.Entry) (*DatastoreRef, error) {
	// Legacy options first
	storagePaths := config.Get().Uploads.StoragePaths
	if len(storagePaths) > 0 {
		log.Warn("Using legacy options to find a datastore")

		if len(storagePaths) == 1 {
			ds, err := storage.GetOrCreateDatastoreOfType(ctx, log, "file", storagePaths[0])
			if err != nil {
				return nil, err
			}
			return newDatastoreRef(ds), nil
		}

		var basePath string
		var pathSize int64
		for i := 0; i < len(storagePaths); i++ {
			currPath := storagePaths[i]
			ds, err := storage.GetOrCreateDatastoreOfType(ctx, log, "file", currPath)
			if err != nil {
				continue
			}

			size, err := estimatedDatastoreSize(ds, ctx, log)
			if err != nil {
				continue
			}

			if basePath == "" || size < pathSize {
				basePath = currPath
				pathSize = size
			}
		}

		if basePath != "" {
			ds, err := storage.GetOrCreateDatastoreOfType(ctx, log, "file", basePath)
			if err != nil {
				return nil, err
			}
			return newDatastoreRef(ds), nil
		}
	}

	// If we haven't found a legacy option, pick a datastore
	log.Info("Finding a suitable datastore to pick for uploads")
	confDatastores := config.Get().DataStores
	mediaStore := storage.GetDatabase().GetMediaStore(ctx, log)

	var targetDs *types.Datastore
	var dsSize int64
	for _, dsConf := range confDatastores {
		if !dsConf.Enabled {
			continue
		}

		ds, err := mediaStore.GetDatastoreByUri(GetUriForDatastore(dsConf))
		if err != nil {
			continue
		}

		size, err := estimatedDatastoreSize(ds, ctx, log)
		if err != nil {
			continue
		}

		if targetDs == nil || size < dsSize {
			logrus.Info("Using ", ds.Uri)
			targetDs = ds
			dsSize = size
		}
	}

	if targetDs != nil {
		return newDatastoreRef(targetDs), nil
	}

	return nil, errors.New("failed to pick a datastore: none available")
}

func estimatedDatastoreSize(ds *types.Datastore, ctx context.Context, log *logrus.Entry) (int64, error) {
	return storage.GetDatabase().GetMetadataStore(ctx, log).GetEstimatedSizeOfDatastore(ds.DatastoreId)
}
