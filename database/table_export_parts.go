package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type DbExportPart struct {
	ExportId    string
	PartNum     int
	SizeBytes   int64
	FileName    string
	DatastoreId string
	Location    string
}

const insertExportPart = "INSERT INTO export_parts (export_id, index, size_bytes, file_name, datastore_id, location) VALUES ($1, $2, $3, $4, $5, $6);"
const deleteExportPartsById = "DELETE FROM export_parts WHERE export_id = $1;"
const selectExportPartsById = "SELECT export_id, index, size_bytes, file_name, datastore_id, location FROM export_parts WHERE export_id = $1;"
const selectExportPartById = "SELECT export_id, index, size_bytes, file_name, datastore_id, location FROM export_parts WHERE export_id = $1 AND index = $2;"

type exportPartsTableStatements struct {
	insertExportPart      *sql.Stmt
	deleteExportPartsById *sql.Stmt
	selectExportPartsById *sql.Stmt
	selectExportPartById  *sql.Stmt
}

type exportPartsTableWithContext struct {
	statements *exportPartsTableStatements
	ctx        rcontext.RequestContext
}

func prepareExportPartsTables(db *sql.DB) (*exportPartsTableStatements, error) {
	var err error
	stmts := &exportPartsTableStatements{}

	if stmts.insertExportPart, err = db.Prepare(insertExportPart); err != nil {
		return nil, fmt.Errorf("error preparing insertExportPart: %w", err)
	}
	if stmts.deleteExportPartsById, err = db.Prepare(deleteExportPartsById); err != nil {
		return nil, fmt.Errorf("error preparing deleteExportPartsById: %w", err)
	}
	if stmts.selectExportPartsById, err = db.Prepare(selectExportPartsById); err != nil {
		return nil, fmt.Errorf("error preparing selectExportPartsById: %w", err)
	}
	if stmts.selectExportPartById, err = db.Prepare(selectExportPartById); err != nil {
		return nil, fmt.Errorf("error preparing selectExportPartById: %w", err)
	}

	return stmts, nil
}

func (s *exportPartsTableStatements) Prepare(ctx rcontext.RequestContext) *exportPartsTableWithContext {
	return &exportPartsTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *exportPartsTableWithContext) GetForExport(exportId string) ([]*DbExportPart, error) {
	results := make([]*DbExportPart, 0)
	rows, err := s.statements.selectExportPartsById.QueryContext(s.ctx, exportId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return results, nil
		}
		return nil, err
	}
	for rows.Next() {
		val := &DbExportPart{}
		if err = rows.Scan(&val.ExportId, &val.PartNum, &val.SizeBytes, &val.FileName, &val.DatastoreId, &val.Location); err != nil {
			return nil, err
		}
		results = append(results, val)
	}
	return results, nil
}

func (s *exportPartsTableWithContext) Get(exportId string, partNum int) (*DbExportPart, error) {
	row := s.statements.selectExportPartById.QueryRowContext(s.ctx, exportId, partNum)
	val := &DbExportPart{}
	err := row.Scan(&val.ExportId, &val.PartNum, &val.SizeBytes, &val.FileName, &val.DatastoreId, &val.Location)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = nil
	}
	return val, err
}

func (s *exportPartsTableWithContext) Insert(part *DbExportPart) error {
	_, err := s.statements.insertExportPart.ExecContext(s.ctx, part.ExportId, part.PartNum, part.SizeBytes, part.FileName, part.DatastoreId, part.Location)
	return err
}

func (s *exportPartsTableWithContext) DeleteForExport(exportId string) error {
	_, err := s.statements.deleteExportPartsById.ExecContext(s.ctx, exportId)
	return err
}
