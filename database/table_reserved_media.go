package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type DbReservedMedia struct {
	Origin  string
	MediaId string
	Reason  string
}

const insertReservedMediaNoConflict = "INSERT INTO reserved_media (origin, media_id, reason) VALUES ($1, $2, $3) ON CONFLICT (origin, media_id) DO NOTHING;"
const selectReservedMediaExists = "SELECT TRUE FROM reserved_media WHERE origin = $1 AND media_id = $2 LIMIT 1;"

type reservedMediaTableStatements struct {
	insertReservedMediaNoConflict *sql.Stmt
	selectReservedMediaExists     *sql.Stmt
}

type reservedMediaTableWithContext struct {
	statements *reservedMediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareReservedMediaTables(db *sql.DB) (*reservedMediaTableStatements, error) {
	var err error
	stmts := &reservedMediaTableStatements{}

	if stmts.insertReservedMediaNoConflict, err = db.Prepare(insertReservedMediaNoConflict); err != nil {
		return nil, fmt.Errorf("error preparing insertReservedMediaNoConflict: %w", err)
	}
	if stmts.selectReservedMediaExists, err = db.Prepare(selectReservedMediaExists); err != nil {
		return nil, fmt.Errorf("error preparing selectReservedMediaExists: %w", err)
	}

	return stmts, nil
}

func (s *reservedMediaTableStatements) Prepare(ctx rcontext.RequestContext) *reservedMediaTableWithContext {
	return &reservedMediaTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *reservedMediaTableWithContext) InsertNoConflict(origin string, mediaId string, reason string) error {
	_, err := s.statements.insertReservedMediaNoConflict.ExecContext(s.ctx, origin, mediaId, reason)
	return err
}

func (s *reservedMediaTableWithContext) IdExists(origin string, mediaId string) (bool, error) {
	row := s.statements.selectReservedMediaExists.QueryRowContext(s.ctx, origin, mediaId)
	val := false
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = false
	}
	return val, err
}
