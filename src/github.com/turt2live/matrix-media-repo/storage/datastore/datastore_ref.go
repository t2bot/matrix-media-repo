package datastore

import (
	"context"
	"errors"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
)

type DatastoreRef struct {
	// TODO: Don't blindly copy properties from types.Datastore
	DatastoreId string
	Type        string
	Uri         string

	datastore *types.Datastore
}

func newDatastoreRef(ds *types.Datastore) *DatastoreRef {
	return &DatastoreRef{
		DatastoreId: ds.DatastoreId,
		Type:        ds.Type,
		Uri:         ds.Uri,
		datastore:   ds,
	}
}

func (d *DatastoreRef) ResolveFilePath(location string) (string) {
	return d.datastore.ResolveFilePath(location)
}

func (d *DatastoreRef) UploadFile(file io.Reader, ctx context.Context, log *logrus.Entry) (string, error) {
	if d.Type == "file" {
		return storage.PersistFile(d.Uri, file, ctx, log)
	} else if d.Type == "s3" {
		panic("failed to upload file")
	} else {
		return "", errors.New("unknown datastore type")
	}
}
