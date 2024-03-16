package database

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/DavidHuie/gomigrate"
	"github.com/getsentry/sentry-go"
	_ "github.com/lib/pq" // postgres driver
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/logging"
)

type Database struct {
	conn            *sql.DB
	Media           *mediaTableStatements
	ExpiringMedia   *expiringMediaTableStatements
	UserStats       *userStatsTableStatements
	ReservedMedia   *reservedMediaTableStatements
	MetadataView    *metadataVirtualTableStatements
	HeldMedia       *heldMediaTableStatements
	Thumbnails      *thumbnailsTableStatements
	LastAccess      *lastAccessTableStatements
	UrlPreviews     *urlPreviewsTableStatements
	MediaAttributes *mediaAttributesTableStatements
	Tasks           *tasksTableStatements
	Exports         *exportsTableStatements
	ExportParts     *exportPartsTableStatements
}

var instance *Database
var singleton = &sync.Once{}

func GetInstance() *Database {
	if instance == nil {
		singleton.Do(func() {
			if err := openDatabase(
				config.Get().Database.Postgres,
				config.Get().Database.Pool.MaxConnections,
				config.Get().Database.Pool.MaxIdle,
			); err != nil {
				logrus.Fatal("Failed to set up database: ", err)
			}
		})
	}
	return instance
}

func Reload() {
	if instance != nil {
		if err := instance.conn.Close(); err != nil {
			logrus.Error(err)
			sentry.CaptureException(err)
		}
	}

	instance = nil
	singleton = &sync.Once{}
	GetInstance()
}

func GetAccessorForTests() *sql.DB {
	return GetInstance().conn
}

func openDatabase(connectionString string, maxConns int, maxIdleConns int) error {
	d := &Database{}
	var err error

	if d.conn, err = sql.Open("postgres", connectionString); err != nil {
		return fmt.Errorf("error connecting to db: %w", err)
	}
	d.conn.SetMaxOpenConns(maxConns)
	d.conn.SetMaxIdleConns(maxIdleConns)

	// Run migrations
	var migrator *gomigrate.Migrator
	if migrator, err = gomigrate.NewMigratorWithLogger(d.conn, gomigrate.Postgres{}, config.Runtime.MigrationsPath, &logging.SendToDebugLogger{}); err != nil {
		return fmt.Errorf("error setting up migrator: %w", err)
	}
	if err = migrator.Migrate(); err != nil {
		return fmt.Errorf("error running migrations: %w", err)
	}

	// Prepare the table accessors
	if d.Media, err = prepareMediaTables(d.conn); err != nil {
		return fmt.Errorf("failed to create media table accessor: %w", err)
	}
	if d.ExpiringMedia, err = prepareExpiringMediaTables(d.conn); err != nil {
		return fmt.Errorf("failed to create expiring media table accessor: %w", err)
	}
	if d.UserStats, err = prepareUserStatsTables(d.conn); err != nil {
		return fmt.Errorf("failed to create user stats table accessor: %w", err)
	}
	if d.ReservedMedia, err = prepareReservedMediaTables(d.conn); err != nil {
		return fmt.Errorf("failed to create reserved media table accessor: %w", err)
	}
	if d.MetadataView, err = prepareMetadataVirtualTables(d.conn); err != nil {
		return fmt.Errorf("failed to create metadata virtual table accessor: %w", err)
	}
	if d.HeldMedia, err = prepareHeldMediaTables(d.conn); err != nil {
		return fmt.Errorf("failed to create held media table accessor: %w", err)
	}
	if d.Thumbnails, err = prepareThumbnailsTables(d.conn); err != nil {
		return fmt.Errorf("failed to create thumbnails table accessor: %w", err)
	}
	if d.LastAccess, err = prepareLastAccessTables(d.conn); err != nil {
		return fmt.Errorf("failed to create last access table accessor: %w", err)
	}
	if d.UrlPreviews, err = prepareUrlPreviewsTables(d.conn); err != nil {
		return fmt.Errorf("failed to create url previews table accessor: %w", err)
	}
	if d.MediaAttributes, err = prepareMediaAttributesTables(d.conn); err != nil {
		return fmt.Errorf("failed to create media attributes table accessor: %w", err)
	}
	if d.Tasks, err = prepareTasksTables(d.conn); err != nil {
		return fmt.Errorf("failed to create tasks table accessor: %w", err)
	}
	if d.Exports, err = prepareExportsTables(d.conn); err != nil {
		return fmt.Errorf("failed to create exports table accessor: %w", err)
	}
	if d.ExportParts, err = prepareExportPartsTables(d.conn); err != nil {
		return fmt.Errorf("failed to create export parts table accessor: %w", err)
	}

	instance = d
	return nil
}
