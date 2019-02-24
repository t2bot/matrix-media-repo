package types

import (
	"path"
)

type Datastore struct {
	DatastoreId string
	Type        string
	Uri         string
}

func (d *Datastore) ResolveFilePath(location string) string {
	return path.Join(d.Uri, location)
}
