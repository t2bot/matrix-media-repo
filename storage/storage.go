package storage

import (
	"database/sql"
	"sync"

	"github.com/DavidHuie/gomigrate"
	_ "github.com/lib/pq" // postgres driver
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage/stores"
)

type Database struct {
	db    *sql.DB
	repos repos
}

type repos struct {
	mediaStore           *stores.MediaStoreFactory
	thumbnailStore       *stores.ThumbnailStoreFactory
	urlStore             *stores.UrlStoreFactory
	metadataStore        *stores.MetadataStoreFactory
	exportStore          *stores.ExportStoreFactory
	mediaAttributesStore *stores.MediaAttributesStoreFactory
}

var dbInstance *Database
var singletonDbLock = &sync.Once{}

func GetDatabase() *Database {
	if dbInstance == nil {
		singletonDbLock.Do(func() {
			err := OpenDatabase(
				config.Get().Database.Postgres,
				config.Get().Database.Pool.MaxConnections,
				config.Get().Database.Pool.MaxIdle)
			if err != nil {
				panic(err)
			}
		})
	}
	return dbInstance
}

func ReloadDatabase() {
	if dbInstance != nil {
		if err := dbInstance.db.Close(); err != nil {
			logrus.Error(err)
		}
	}

	dbInstance = nil
	singletonDbLock = &sync.Once{}
	GetDatabase()
}

func OpenDatabase(connectionString string, maxConns int, maxIdleConns int) error {
	d := &Database{}
	var err error

	if d.db, err = sql.Open("postgres", connectionString); err != nil {
		return err
	}
	d.db.SetMaxOpenConns(maxConns)
	d.db.SetMaxIdleConns(maxIdleConns)

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
	logrus.Info("Setting up media DB store...")
	if d.repos.mediaStore, err = stores.InitMediaStore(d.db); err != nil {
		return err
	}
	logrus.Info("Setting up thumbnails DB store...")
	if d.repos.thumbnailStore, err = stores.InitThumbnailStore(d.db); err != nil {
		return err
	}
	logrus.Info("Setting up URL previews DB store...")
	if d.repos.urlStore, err = stores.InitUrlStore(d.db); err != nil {
		return err
	}
	logrus.Info("Setting up metadata DB store...")
	if d.repos.metadataStore, err = stores.InitMetadataStore(d.db); err != nil {
		return err
	}
	logrus.Info("Setting up export DB store...")
	if d.repos.exportStore, err = stores.InitExportStore(d.db); err != nil {
		return err
	}
	logrus.Info("Setting up media attributes DB store...")
	if d.repos.mediaAttributesStore, err = stores.InitMediaAttributesStore(d.db); err != nil {
		return err
	}

	// Run some tasks that should always be done on startup
	if err = populateDatastores(d); err != nil {
		return err
	}
	if err = populateThumbnailHashes(d); err != nil {
		return err
	}

	dbInstance = d
	return nil
}

func (d *Database) GetMediaStore(ctx rcontext.RequestContext) *stores.MediaStore {
	return d.repos.mediaStore.Create(ctx)
}

func (d *Database) GetThumbnailStore(ctx rcontext.RequestContext) *stores.ThumbnailStore {
	return d.repos.thumbnailStore.New(ctx)
}

func (d *Database) GetUrlStore(ctx rcontext.RequestContext) *stores.UrlStore {
	return d.repos.urlStore.Create(ctx)
}

func (d *Database) GetMetadataStore(ctx rcontext.RequestContext) *stores.MetadataStore {
	return d.repos.metadataStore.Create(ctx)
}

func (d *Database) GetExportStore(ctx rcontext.RequestContext) *stores.ExportStore {
	return d.repos.exportStore.Create(ctx)
}

func (d *Database) GetMediaAttributesStore(ctx rcontext.RequestContext) *stores.MediaAttributesStore {
	return d.repos.mediaAttributesStore.Create(ctx)
}
