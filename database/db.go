package database

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/DavidHuie/gomigrate"
	"github.com/getsentry/sentry-go"
	_ "github.com/lib/pq" // postgres driver
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
)

type Database struct {
	conn          *sql.DB
	Media         *mediaTableStatements
	ExpiringMedia *expiringMediaTableStatements
	UserStats     *userStatsTableStatements
	ReservedMedia *reservedMediaTableStatements
	MetadataView  *metadataVirtualTableStatements
	Blurhashes    *blurhashesTableStatements
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

func openDatabase(connectionString string, maxConns int, maxIdleConns int) error {
	d := &Database{}
	var err error

	if d.conn, err = sql.Open("postgres", connectionString); err != nil {
		return errors.New("error connecting to db: " + err.Error())
	}
	d.conn.SetMaxOpenConns(maxConns)
	d.conn.SetMaxIdleConns(maxIdleConns)

	// Run migrations
	var migrator *gomigrate.Migrator
	if migrator, err = gomigrate.NewMigratorWithLogger(d.conn, gomigrate.Postgres{}, config.Runtime.MigrationsPath, &logging.SendToDebugLogger{}); err != nil {
		return errors.New("error setting up migrator: " + err.Error())
	}
	if err = migrator.Migrate(); err != nil {
		return errors.New("error running migrations: " + err.Error())
	}

	// Prepare the table accessors
	if d.Media, err = prepareMediaTables(d.conn); err != nil {
		return errors.New("failed to create media table accessor: " + err.Error())
	}
	if d.ExpiringMedia, err = prepareExpiringMediaTables(d.conn); err != nil {
		return errors.New("failed to create expiring media table accessor: " + err.Error())
	}
	if d.UserStats, err = prepareUserStatsTables(d.conn); err != nil {
		return errors.New("failed to create user stats table accessor: " + err.Error())
	}
	if d.ReservedMedia, err = prepareReservedMediaTables(d.conn); err != nil {
		return errors.New("failed to create reserved media table accessor: " + err.Error())
	}
	if d.MetadataView, err = prepareMetadataVirtualTables(d.conn); err != nil {
		return errors.New("failed to create metadata virtual table accessor: " + err.Error())
	}
	if d.Blurhashes, err = prepareBlurhashesTables(d.conn); err != nil {
		return errors.New("failed to create blurhashes table accessor: " + err.Error())
	}

	instance = d
	return nil
}
