package database

import (
	"database/sql"
	"errors"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
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

const insertHeldMedia = "INSERT INTO media_id_hold (origin, media_id, reason, held_ts) VALUES ($1, $2, $3, $4);"
const deleteHeldMedia = "DELETE FROM media_id_hold WHERE reason = $1 AND held_ts <= $2;"

type heldMediaTableStatements struct {
	insertHeldMedia *sql.Stmt
	deleteHeldMedia *sql.Stmt
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
	if stmts.deleteHeldMedia, err = db.Prepare(deleteHeldMedia); err != nil {
		return nil, errors.New("error preparing deleteHeldMedia: " + err.Error())
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
	_, err := s.statements.insertHeldMedia.ExecContext(s.ctx, origin, mediaId, reason, util.NowMillis())
	return err
}

func (s *heldMediaTableWithContext) DeleteOlderThan(reason HeldReason, olderThanTs int64) error {
	_, err := s.statements.deleteHeldMedia.ExecContext(s.ctx, reason, olderThanTs)
	return err
}
