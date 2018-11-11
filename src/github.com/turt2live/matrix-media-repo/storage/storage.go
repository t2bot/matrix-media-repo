package storage

import (
	"context"
	"database/sql"
	"sync"

	"github.com/DavidHuie/gomigrate"
	_ "github.com/lib/pq" // postgres driver
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/storage/stores"
)

type Database struct {
	db    *sql.DB
	repos repos
}

type repos struct {
	mediaStore     *stores.MediaStoreFactory
	thumbnailStore *stores.ThumbnailStoreFactory
	urlStore       *stores.UrlStoreFactory
	metadataStore  *stores.MetadataStoreFactory
}

var dbInstance *Database
var singletonDbLock = &sync.Once{}

func GetDatabase() (*Database) {
	if dbInstance == nil {
		singletonDbLock.Do(func() {
			err := OpenDatabase(config.Get().Database.Postgres)
			if err != nil {
				panic(err)
			}
		})
	}
	return dbInstance
}

func OpenDatabase(connectionString string) (error) {
	d := &Database{}
	var err error

	if d.db, err = sql.Open("postgres", connectionString); err != nil {
		return err
	}

	// Make sure the database is how we want it
	migrator, err := gomigrate.NewMigratorWithLogger(d.db, gomigrate.Postgres{}, config.Runtime.MigrationsPath, logrus.StandardLogger())
	if err != nil {
		return err
	}
	err = migrator.Migrate()
	if err != nil {
		return err
	}

	// New the repo factories
	if d.repos.mediaStore, err = stores.InitMediaStore(d.db); err != nil {
		return err
	}
	if d.repos.thumbnailStore, err = stores.InitThumbnailStore(d.db); err != nil {
		return err
	}
	if d.repos.urlStore, err = stores.InitUrlStore(d.db); err != nil {
		return err
	}
	if d.repos.metadataStore, err = stores.InitMetadataStore(d.db); err != nil {
		return err
	}

	// Run some tasks that should always be done on startup
	if err = populateThumbnailHashes(d); err != nil {
		return err
	}

	dbInstance = d
	return nil
}

func (d *Database) GetMediaStore(ctx context.Context, log *logrus.Entry) (*stores.MediaStore) {
	return d.repos.mediaStore.Create(ctx, log)
}

func (d *Database) GetThumbnailStore(ctx context.Context, log *logrus.Entry) (*stores.ThumbnailStore) {
	return d.repos.thumbnailStore.New(ctx, log)
}

func (d *Database) GetUrlStore(ctx context.Context, log *logrus.Entry) (*stores.UrlStore) {
	return d.repos.urlStore.Create(ctx, log)
}

func (d *Database) GetMetadataStore(ctx context.Context, log *logrus.Entry) (*stores.MetadataStore) {
	return d.repos.metadataStore.Create(ctx, log)
}
