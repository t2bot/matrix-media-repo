package datastore

import (
	"context"
	"errors"
	"io"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_file"
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
	config    config.DatastoreConfig
}

func newDatastoreRef(ds *types.Datastore, config config.DatastoreConfig) *DatastoreRef {
	return &DatastoreRef{
		DatastoreId: ds.DatastoreId,
		Type:        ds.Type,
		Uri:         ds.Uri,
		datastore:   ds,
		config:      config,
	}
}

func (d *DatastoreRef) UploadFile(file io.ReadCloser, ctx context.Context, log *logrus.Entry) (*types.ObjectInfo, error) {
	log = log.WithFields(logrus.Fields{"datastoreId": d.DatastoreId, "datastoreUri": d.Uri})

	if d.Type == "file" {
		return ds_file.PersistFile(d.Uri, file, ctx, log)
	} else if d.Type == "s3" {
		s3, err := ds_s3.GetOrCreateS3Datastore(d.DatastoreId, d.config)
		if err != nil {
			return nil, err
		}
		return s3.UploadFile(file, ctx, log)
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
	} else {
		panic("unknown datastore type")
	}
}

func (d *DatastoreRef) OverwriteObject(location string, stream io.ReadCloser, ctx context.Context, log *logrus.Entry) error {
	if d.Type == "file" {
		_, _, err := ds_file.PersistFileAtLocation(path.Join(d.Uri, location), stream, ctx, log)
		return err
	} else if d.Type == "s3" {
		s3, err := ds_s3.GetOrCreateS3Datastore(d.DatastoreId, d.config)
		if err != nil {
			return err
		}
		return s3.OverwriteObject(location, stream)
	} else {
		return errors.New("unknown datastore type")
	}
}
