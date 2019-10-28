package stores

import (
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
)

const insertExportMetadata = "INSERT INTO exports (export_id, entity) VALUES ($1, $2);"
const insertExportPart = "INSERT INTO export_parts (export_id, index, size_bytes, file_name, datastore_id, location) VALUES ($1, $2, $3, $4, $5, $6);"
const selectExportMetadata = "SELECT export_id, entity FROM exports WHERE export_id = $1;"
const selectExportParts = "SELECT export_id, index, size_bytes, file_name, datastore_id, location FROM export_parts WHERE export_id = $1;"
const selectExportPart = "SELECT export_id, index, size_bytes, file_name, datastore_id, location FROM export_parts WHERE export_id = $1 AND index = $2;"
const deleteExportParts = "DELETE FROM export_parts WHERE export_id = $1;"
const deleteExport = "DELETE FROM exports WHERE export_id = $1;"

type exportStoreStatements struct {
	insertExportMetadata *sql.Stmt
	insertExportPart     *sql.Stmt
	selectExportMetadata *sql.Stmt
	selectExportParts    *sql.Stmt
	selectExportPart     *sql.Stmt
	deleteExportParts    *sql.Stmt
	deleteExport         *sql.Stmt
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
	if store.stmts.selectExportMetadata, err = store.sqlDb.Prepare(selectExportMetadata); err != nil {
		return nil, err
	}
	if store.stmts.selectExportParts, err = store.sqlDb.Prepare(selectExportParts); err != nil {
		return nil, err
	}
	if store.stmts.selectExportPart, err = store.sqlDb.Prepare(selectExportPart); err != nil {
		return nil, err
	}
	if store.stmts.deleteExportParts, err = store.sqlDb.Prepare(deleteExportParts); err != nil {
		return nil, err
	}
	if store.stmts.deleteExport, err = store.sqlDb.Prepare(deleteExport); err != nil {
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

func (s *ExportStore) GetExportMetadata(exportId string) (*types.ExportMetadata, error) {
	m := &types.ExportMetadata{}
	err := s.statements.selectExportMetadata.QueryRowContext(s.ctx, exportId).Scan(
		&m.ExportID,
		&m.Entity,
	)
	return m, err
}

func (s *ExportStore) GetExportParts(exportId string) ([]*types.ExportPart, error) {
	rows, err := s.statements.selectExportParts.QueryContext(s.ctx, exportId)
	if err != nil {
		return nil, err
	}

	var results []*types.ExportPart
	for rows.Next() {
		obj := &types.ExportPart{}
		err = rows.Scan(
			&obj.ExportID,
			&obj.Index,
			&obj.SizeBytes,
			&obj.FileName,
			&obj.DatastoreID,
			&obj.Location,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *ExportStore) GetExportPart(exportId string, index int) (*types.ExportPart, error) {
	m := &types.ExportPart{}
	err := s.statements.selectExportPart.QueryRowContext(s.ctx, exportId, index).Scan(
		&m.ExportID,
		&m.Index,
		&m.SizeBytes,
		&m.FileName,
		&m.DatastoreID,
		&m.Location,
	)
	return m, err
}

func (s *ExportStore) DeleteExportAndParts(exportId string) error {
	_, err := s.statements.deleteExportParts.ExecContext(s.ctx, exportId)
	if err != nil {
		return err
	}

	_, err = s.statements.deleteExport.ExecContext(s.ctx, exportId)
	if err != nil {
		return err
	}

	return nil
}
