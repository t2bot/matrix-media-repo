package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbHeldMedia struct {
	Origin  string
	MediaId string
	Reason  string
}

type HeldReason string

const (
	ForCreateHeldReason HeldReason = "media_create"
)

const insertHeldMedia = "INSERT INTO media_id_hold (origin, media_id, reason) VALUES ($1, $2, $3);"

type heldMediaTableStatements struct {
	insertHeldMedia *sql.Stmt
}

type heldMediaTableWithContext struct {
	statements *heldMediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareHeldMediaTables(db *sql.DB) (*heldMediaTableStatements, error) {
	var err error
	var stmts = &heldMediaTableStatements{}

	if stmts.insertHeldMedia, err = db.Prepare(insertHeldMedia); err != nil {
		return nil, errors.New("error preparing insertHeldMedia: " + err.Error())
	}

	return stmts, nil
}

func (s *heldMediaTableStatements) Prepare(ctx rcontext.RequestContext) *heldMediaTableWithContext {
	return &heldMediaTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *heldMediaTableWithContext) TryInsert(origin string, mediaId string, reason HeldReason) error {
	_, err := s.statements.insertHeldMedia.ExecContext(s.ctx, origin, mediaId, reason)
	return err
}
