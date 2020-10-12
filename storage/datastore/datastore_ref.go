package datastore

import (
	"errors"
	"io"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	config2 "github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_file"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_ipfs"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_s3"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type DatastoreRef struct {
	// TODO: Don't blindly copy properties from types.Datastore
	DatastoreId string
	Type        string
	Uri         string

	datastore *types.Datastore
	config    config2.DatastoreConfig
}

func newDatastoreRef(ds *types.Datastore, config config2.DatastoreConfig) *DatastoreRef {
	return &DatastoreRef{
		DatastoreId: ds.DatastoreId,
		Type:        ds.Type,
		Uri:         ds.Uri,
		datastore:   ds,
		config:      config,
	}
}

func (d *DatastoreRef) UploadFile(file io.ReadCloser, expectedLength int64, ctx rcontext.RequestContext) (*types.ObjectInfo, error) {
	ctx = ctx.LogWithFields(logrus.Fields{"datastoreId": d.DatastoreId, "datastoreUri": d.Uri})

	if d.Type == "file" {
		return ds_file.PersistFile(d.Uri, file, ctx)
	} else if d.Type == "s3" {
		s3, err := ds_s3.GetOrCreateS3Datastore(d.DatastoreId, d.config)
		if err != nil {
			return nil, err
		}
		return s3.UploadFile(file, expectedLength, ctx)
	} else if d.Type == "gcp" {
		gcp, err := ds_s3.GetOrCreateGCPDatastore(d.DatastoreId, d.config)
		if err != nil {
			return nil, err
		}
		return gcp.UploadFile(file, expectedLength, ctx)
	} else if d.Type == "ipfs" {
		return ds_ipfs.UploadFile(file, ctx)
	} else {
		return nil, errors.New("unknown datastore type")
	}
}

func (d *DatastoreRef) DeleteObject(location string) error {
	if d.Type == "file" {
		return ds_file.DeletePersistedFile(d.Uri, location)
	} else if d.Type == "s3" {
		s3, err := ds_s3.GetOrCreateS3Datastore(d.DatastoreId, d.config)
		if err != nil {
			return err
		}
		return s3.DeleteObject(location)
	} else if d.Type == "gcp" {
		gcp, err := ds_s3.GetOrCreateGCPDatastore(d.Uri, d.config)
		if err != nil {
			return err
		}
		return gcp.DeleteObject(location)
	} else if d.Type == "ipfs" {
		// TODO: Support deleting from IPFS - will need a "delete reason" to avoid deleting duplicates
		logrus.Warn("Unsupported operation: deleting from IPFS datastore")
		return nil
	} else {
		return errors.New("unknown datastore type")
	}
}

func (d *DatastoreRef) DownloadFile(location string) (io.ReadCloser, error) {
	if d.Type == "file" {
		return os.Open(path.Join(d.Uri, location))
	} else if d.Type == "s3" {
		s3, err := ds_s3.GetOrCreateS3Datastore(d.DatastoreId, d.config)
		if err != nil {
			return nil, err
		}
		return s3.DownloadObject(location)
	} else if d.Type == "gcp" {
		gcp, err := ds_s3.GetOrCreateGCPDatastore(d.DatastoreId, d.config)
		if err != nil {
			return nil, err
		}
		return gcp.DownloadObject(location)
	} else if d.Type == "ipfs" {
		return ds_ipfs.DownloadFile(location)
	} else {
		return nil, errors.New("unknown datastore type")
	}
}

func (d *DatastoreRef) ObjectExists(location string) bool {
	if d.Type == "file" {
		ok, err := util.FileExists(path.Join(d.Uri, location))
		if err != nil {
			return false
		}
		return ok
	} else if d.Type == "s3" {
		s3, err := ds_s3.GetOrCreateS3Datastore(d.DatastoreId, d.config)
		if err != nil {
			return false
		}
		return s3.ObjectExists(location)
	} else if d.Type == "gcp" {
		gcp, err := ds_s3.GetOrCreateGCPDatastore(d.DatastoreId, d.config)
		if err != nil {
			return false
		}
		return gcp.ObjectExists(location)
	} else if d.Type == "ipfs" {
		// TODO: Support checking file existence in IPFS
		logrus.Warn("Unsupported operation: existence in IPFS datastore")
		return false
	} else {
		panic("unknown datastore type")
	}
}

func (d *DatastoreRef) OverwriteObject(location string, stream io.ReadCloser, ctx rcontext.RequestContext) error {
	if d.Type == "file" {
		_, _, err := ds_file.PersistFileAtLocation(path.Join(d.Uri, location), stream, ctx)
		return err
	} else if d.Type == "s3" {
		s3, err := ds_s3.GetOrCreateS3Datastore(d.DatastoreId, d.config)
		if err != nil {
			return err
		}
		return s3.OverwriteObject(location, stream)
	} else if d.Type == "gcp" {
		gcp, err := ds_s3.GetOrCreateGCPDatastore(d.DatastoreId, d.config)
		if err != nil {
			return err
		}
		return gcp.OverwriteObject(location, stream)
	} else if d.Type == "ipfs" {
		// TODO: Support overwriting in IPFS
		logrus.Warn("Unsupported operation: overwriting file in IPFS datastore")
		return errors.New("unsupported operation")
	} else {
		return errors.New("unknown datastore type")
	}
}
