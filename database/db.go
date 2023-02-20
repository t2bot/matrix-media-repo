package database

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/DavidHuie/gomigrate"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
)

type Database struct {
	conn       *sql.DB
	Datastores *dsTableStatements
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
	if migrator, err = gomigrate.NewMigratorWithLogger(d.conn, gomigrate.Postgres{}, config.Runtime.MigrationsPath, logrus.StandardLogger()); err != nil {
		return errors.New("error setting up migrator: " + err.Error())
	}
	if err = migrator.Migrate(); err != nil {
		return errors.New("error running migrations: " + err.Error())
	}

	// Prepare the table accessors
	if d.Datastores, err = prepareDatastoreTables(d.conn); err != nil {
		return errors.New("failed to create datastores table accessor")
	}

	return nil
}
