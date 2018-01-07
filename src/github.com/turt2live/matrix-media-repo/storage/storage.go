package storage

import (
	"context"
	"database/sql"

	_ "github.com/lib/pq" // postgres driver
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/storage/schema"
	"github.com/turt2live/matrix-media-repo/storage/stores"
)

const selectSizeOfFolder = "SELECT COALESCE(SUM(size_bytes), 0) + COALESCE((SELECT SUM(size_bytes) FROM thumbnails WHERE location ILIKE $1 || '%'), 0) AS size_total FROM media WHERE location ILIKE $1 || '%';"

type folderSize struct {
	Size int64
}

type Database struct {
	db         *sql.DB
	statements statements
	repos      repos
}

type statements struct {
	selectSizeOfFolder *sql.Stmt
}

type repos struct {
	mediaStore     *stores.MediaStoreFactory
	thumbnailStore *stores.ThumbnailStoreFactory
	urlStore       *stores.UrlStoreFactory
}

func OpenDatabase(connectionString string) (*Database, error) {
	var d Database
	var err error

	if d.db, err = sql.Open("postgres", connectionString); err != nil {
		return nil, err
	}

	// Make sure the database is how we want it
	schema.PrepareMedia(d.db)
	schema.PrepareThumbnails(d.db)
	schema.PrepareUrls(d.db)

	// Create the repo factories
	if d.repos.mediaStore, err = stores.InitMediaStore(d.db); err != nil {
		return nil, err
	}
	if d.repos.thumbnailStore, err = stores.InitThumbnailStore(d.db); err != nil {
		return nil, err
	}
	if d.repos.urlStore, err = stores.InitUrlStore(d.db); err != nil {
		return nil, err
	}

	// Prepare the general statements
	if d.statements.selectSizeOfFolder, err = d.db.Prepare(selectSizeOfFolder); err != nil {
		return nil, err
	}

	return &d, nil
}

func (d *Database) GetMediaStore(ctx context.Context, log *logrus.Entry) (*stores.MediaStore) {
	return d.repos.mediaStore.Create(ctx, log)
}

func (d *Database) GetThumbnailStore(ctx context.Context, log *logrus.Entry) (*stores.ThumbnailStore) {
	return d.repos.thumbnailStore.Create(ctx, log)
}

func (d *Database) GetUrlStore(ctx context.Context, log *logrus.Entry) (*stores.UrlStore) {
	return d.repos.urlStore.Create(ctx, log)
}

func (d *Database) GetSizeOfFolderBytes(ctx context.Context, folderPath string) (int64, error) {
	r := &folderSize{}
	err := d.statements.selectSizeOfFolder.QueryRowContext(ctx, folderPath).Scan(&r.Size)
	return r.Size, err
}
