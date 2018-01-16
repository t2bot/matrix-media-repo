package storage

import (
	"context"
	"database/sql"
	"sync"

	"github.com/DavidHuie/gomigrate"
	_ "github.com/lib/pq" // postgres driver
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
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

	// Prepare the general statements
	if d.statements.selectSizeOfFolder, err = d.db.Prepare(selectSizeOfFolder); err != nil {
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

func (d *Database) GetSizeOfFolderBytes(ctx context.Context, folderPath string) (int64, error) {
	r := &folderSize{}
	err := d.statements.selectSizeOfFolder.QueryRowContext(ctx, folderPath).Scan(&r.Size)
	return r.Size, err
}
