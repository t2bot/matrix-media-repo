package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbReservedMedia struct {
	Origin  string
	MediaId string
	Reason  string
}

const insertReservedMedia = "INSERT INTO reserved_media (origin, media_id, reason) VALUES ($1, $2, $3);"

type reservedMediaTableStatements struct {
	insertReservedMedia *sql.Stmt
}

type reservedMediaTableWithContext struct {
	statements *reservedMediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareReservedMediaTables(db *sql.DB) (*reservedMediaTableStatements, error) {
	var err error
	var stmts = &reservedMediaTableStatements{}

	if stmts.insertReservedMedia, err = db.Prepare(insertReservedMedia); err != nil {
		return nil, errors.New("error preparing insertReservedMedia: " + err.Error())
	}

	return stmts, nil
}

func (s *reservedMediaTableStatements) Prepare(ctx rcontext.RequestContext) *reservedMediaTableWithContext {
	return &reservedMediaTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *reservedMediaTableWithContext) TryInsert(origin string, mediaId string, reason string) error {
	_, err := s.statements.insertReservedMedia.ExecContext(s.ctx, origin, mediaId, reason)
	return err
}
