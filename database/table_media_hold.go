package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
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
	stmts := &heldMediaTableStatements{}

	if stmts.insertHeldMedia, err = db.Prepare(insertHeldMedia); err != nil {
		return nil, fmt.Errorf("error preparing insertHeldMedia: %w", err)
	}
	if stmts.deleteHeldMedia, err = db.Prepare(deleteHeldMedia); err != nil {
		return nil, fmt.Errorf("error preparing deleteHeldMedia: %w", err)
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
	_, err := s.statements.insertHeldMedia.ExecContext(s.ctx, origin, mediaId, reason, time.Now().UnixMilli())
	return err
}

func (s *heldMediaTableWithContext) DeleteOlderThan(reason HeldReason, olderThan time.Time) error {
	_, err := s.statements.deleteHeldMedia.ExecContext(s.ctx, reason, olderThan.UnixMilli())
	return err
}
