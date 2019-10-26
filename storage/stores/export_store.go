package stores

import (
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
)

const insertExportMetadata = "INSERT INTO exports (export_id, entity) VALUES ($1, $2);"
const insertExportPart = "INSERT INTO export_parts (export_id, index, size_bytes, file_name, datastore_id, location) VALUES ($1, $2, $3, $4, $5, $6);"

type exportStoreStatements struct {
	insertExportMetadata *sql.Stmt
	insertExportPart     *sql.Stmt
}

type ExportStoreFactory struct {
	sqlDb *sql.DB
	stmts *exportStoreStatements
}

type ExportStore struct {
	factory    *ExportStoreFactory // just for reference
	ctx        context.Context
	log        *logrus.Entry
	statements *exportStoreStatements // copied from factory
}

func InitExportStore(sqlDb *sql.DB) (*ExportStoreFactory, error) {
	store := ExportStoreFactory{stmts: &exportStoreStatements{}}
	var err error

	store.sqlDb = sqlDb

	if store.stmts.insertExportMetadata, err = store.sqlDb.Prepare(insertExportMetadata); err != nil {
		return nil, err
	}
	if store.stmts.insertExportPart, err = store.sqlDb.Prepare(insertExportPart); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *ExportStoreFactory) Create(ctx context.Context, entry *logrus.Entry) *ExportStore {
	return &ExportStore{
		factory:    f,
		ctx:        ctx,
		log:        entry,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *ExportStore) InsertExport(exportId string, entity string) error {
	_, err := s.statements.insertExportMetadata.ExecContext(s.ctx, exportId, entity)
	return err
}

func (s *ExportStore) InsertExportPart(exportId string, index int, size int64, name string, datastoreId string, location string) error {
	_, err := s.statements.insertExportPart.ExecContext(s.ctx, exportId, index, size, name, datastoreId, location)
	return err
}
