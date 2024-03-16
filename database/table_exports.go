package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

const insertExport = "INSERT INTO exports (export_id, entity) VALUES ($1, $2);"
const selectExportEntity = "SELECT entity FROM exports WHERE export_id = $1;"
const deleteExport = "DELETE FROM exports WHERE export_id = $1;"

type exportsTableStatements struct {
	insertExport       *sql.Stmt
	selectExportEntity *sql.Stmt
	deleteExport       *sql.Stmt
}

type exportsTableWithContext struct {
	statements *exportsTableStatements
	ctx        rcontext.RequestContext
}

func prepareExportsTables(db *sql.DB) (*exportsTableStatements, error) {
	var err error
	stmts := &exportsTableStatements{}

	if stmts.insertExport, err = db.Prepare(insertExport); err != nil {
		return nil, fmt.Errorf("error preparing insertExport: %w", err)
	}
	if stmts.selectExportEntity, err = db.Prepare(selectExportEntity); err != nil {
		return nil, fmt.Errorf("error preparing selectExportEntity: %w", err)
	}
	if stmts.deleteExport, err = db.Prepare(deleteExport); err != nil {
		return nil, fmt.Errorf("error preparing deleteExport: %w", err)
	}

	return stmts, nil
}

func (s *exportsTableStatements) Prepare(ctx rcontext.RequestContext) *exportsTableWithContext {
	return &exportsTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *exportsTableWithContext) Insert(exportId string, entity string) error {
	_, err := s.statements.insertExport.ExecContext(s.ctx, exportId, entity)
	return err
}

func (s *exportsTableWithContext) Delete(exportId string) error {
	_, err := s.statements.deleteExport.ExecContext(s.ctx, exportId)
	return err
}

func (s *exportsTableWithContext) GetEntity(exportId string) (string, error) {
	row := s.statements.selectExportEntity.QueryRowContext(s.ctx, exportId)
	val := ""
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = ""
	}
	return val, err
}
