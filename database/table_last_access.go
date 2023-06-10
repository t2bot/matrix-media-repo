package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbLastAccess struct {
	Sha256Hash   string
	LastAccessTs int64
}

const upsertLastAccess = "INSERT INTO last_access (sha256_hash, last_access_ts) VALUES ($1, $2) ON CONFLICT (sha256_hash) DO UPDATE SET last_access_ts = $2;"

type lastAccessTableStatements struct {
	upsertLastAccess *sql.Stmt
}

type lastAccessTableWithContext struct {
	statements *lastAccessTableStatements
	ctx        rcontext.RequestContext
}

func prepareLastAccessTables(db *sql.DB) (*lastAccessTableStatements, error) {
	var err error
	var stmts = &lastAccessTableStatements{}

	if stmts.upsertLastAccess, err = db.Prepare(upsertLastAccess); err != nil {
		return nil, errors.New("error preparing upsertLastAccess: " + err.Error())
	}

	return stmts, nil
}

func (s *lastAccessTableStatements) Prepare(ctx rcontext.RequestContext) *lastAccessTableWithContext {
	return &lastAccessTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *lastAccessTableWithContext) Upsert(sha256hash string, ts int64) error {
	_, err := s.statements.upsertLastAccess.ExecContext(s.ctx, sha256hash, ts)
	return err
}
