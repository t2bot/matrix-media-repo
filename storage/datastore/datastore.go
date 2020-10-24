package datastore

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
)

func GetAvailableDatastores(ctx rcontext.RequestContext) ([]*types.Datastore, error) {
	datastores := make([]*types.Datastore, 0)
	for _, ds := range ctx.Config.DataStores {
		if !ds.Enabled {
			continue
		}

		uri := GetUriForDatastore(ds)

		dsInstance, err := storage.GetOrCreateDatastoreOfType(rcontext.Initial(), ds.Type, uri)
		if err != nil {
			return nil, err
		}

		datastores = append(datastores, dsInstance)
	}

	return datastores, nil
}

func LocateDatastore(ctx rcontext.RequestContext, datastoreId string) (*DatastoreRef, error) {
	ds, err := storage.GetDatabase().GetMediaStore(ctx).GetDatastore(datastoreId)
	if err != nil {
		return nil, err
	}

	conf, err := GetDatastoreConfig(ds)
	if err != nil {
		return nil, err
	}

	return newDatastoreRef(ds, conf), nil
}

func DownloadStream(ctx rcontext.RequestContext, datastoreId string, location string) (io.ReadCloser, error) {
	ref, err := LocateDatastore(ctx, datastoreId)
	if err != nil {
		return nil, err
	}
	return ref.DownloadFile(location)
}

func GetDatastoreConfig(ds *types.Datastore) (config.DatastoreConfig, error) {
	for _, dsConf := range config.UniqueDatastores() {
		if dsConf.Type == ds.Type && GetUriForDatastore(dsConf) == ds.Uri {
			return dsConf, nil
		}
	}

	return config.DatastoreConfig{}, errors.New("datastore not found")
}

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
		region, regionFound := dsConf.Options["region"]
		if !epFound || !bucketFound {
			logrus.Fatal("Missing 'endpoint' or 'bucketName' on s3 datastore")
		}
		if regionFound {
			return fmt.Sprintf("s3://%s/%s?region=%s", endpoint, bucket, region)
		} else {
			return fmt.Sprintf("s3://%s/%s", endpoint, bucket)
		}
	} else if dsConf.Type == "ipfs" {
		return "ipfs://localhost"
	} else if dsConf.Type == "gcp" {
		jsonPath := dsConf.Options["jsonPath"]
		_, err := os.Open(jsonPath)
		if err != nil {
			logrus.Fatal("could not find json file with given path!")
		}
		return jsonPath
	} else {
		logrus.Fatal("Unknown datastore type: ", dsConf.Type)
	}

	return ""
}

func PickDatastore(forKind string, ctx rcontext.RequestContext) (*DatastoreRef, error) {
	ctx.Log.Info("Finding a suitable datastore to pick for " + forKind)
	confDatastores := ctx.Config.DataStores
	mediaStore := storage.GetDatabase().GetMediaStore(ctx)

	var targetDs *types.Datastore
	var targetDsConf config.DatastoreConfig
	var dsSize int64
	for _, dsConf := range confDatastores {
		if !dsConf.Enabled {
			continue
		}

		allowed := common.HasKind(dsConf.MediaKinds, forKind)
		if !allowed {
			continue
		}

		ds, err := mediaStore.GetDatastoreByUri(GetUriForDatastore(dsConf))
		if err != nil {
			continue
		}

		var size int64

		if len(confDatastores) > 1 {
			size, err = estimatedDatastoreSize(ds, ctx)
			if err != nil {
				continue
			}
		}

		if targetDs == nil || size < dsSize {
			targetDs = ds
			targetDsConf = dsConf
			dsSize = size
		}
	}

	if targetDs != nil {
		ctx.Log.Info("Using ", targetDs.Uri)
		return newDatastoreRef(targetDs, targetDsConf), nil
	}

	return nil, errors.New("failed to pick a datastore: none available")
}

func estimatedDatastoreSize(ds *types.Datastore, ctx rcontext.RequestContext) (int64, error) {
	return storage.GetDatabase().GetMetadataStore(ctx).GetEstimatedSizeOfDatastore(ds.DatastoreId)
}
