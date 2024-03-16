package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type DbUserStats struct {
	UserId        string
	UploadedBytes int64
}

const selectUserStatsUploadedBytes = "SELECT uploaded_bytes FROM user_stats WHERE user_id = $1;"

type userStatsTableStatements struct {
	selectUserStatsUploadedBytes *sql.Stmt
}

type userStatsTableWithContext struct {
	statements *userStatsTableStatements
	ctx        rcontext.RequestContext
}

func prepareUserStatsTables(db *sql.DB) (*userStatsTableStatements, error) {
	var err error
	stmts := &userStatsTableStatements{}

	if stmts.selectUserStatsUploadedBytes, err = db.Prepare(selectUserStatsUploadedBytes); err != nil {
		return nil, fmt.Errorf("error preparing selectUserStatsUploadedBytes: %w", err)
	}

	return stmts, nil
}

func (s *userStatsTableStatements) Prepare(ctx rcontext.RequestContext) *userStatsTableWithContext {
	return &userStatsTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *userStatsTableWithContext) UserUploadedBytes(userId string) (int64, error) {
	row := s.statements.selectUserStatsUploadedBytes.QueryRowContext(s.ctx, userId)
	val := int64(0)
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = 0
	}
	return val, err
}
