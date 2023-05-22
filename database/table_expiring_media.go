package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

type DbExpiringMedia struct {
	Origin    string
	MediaId   string
	UserId    string
	ExpiresTs int64
}

const insertExpiringMedia = "INSERT INTO expiring_media (origin, media_id, user_id, expires_ts) VALUES ($1, $2, $3, $4);"
const selectExpiringMediaByUserCount = "SELECT COUNT(*) FROM expiring_media WHERE user_id = $1 AND expires_ts >= $2;"

type expiringMediaTableStatements struct {
	insertExpiringMedia            *sql.Stmt
	selectExpiringMediaByUserCount *sql.Stmt
}

type expiringMediaTableWithContext struct {
	statements *expiringMediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareExpiringMediaTables(db *sql.DB) (*expiringMediaTableStatements, error) {
	var err error
	var stmts = &expiringMediaTableStatements{}

	if stmts.insertExpiringMedia, err = db.Prepare(insertExpiringMedia); err != nil {
		return nil, errors.New("error preparing insertExpiringMedia: " + err.Error())
	}
	if stmts.selectExpiringMediaByUserCount, err = db.Prepare(selectExpiringMediaByUserCount); err != nil {
		return nil, errors.New("error preparing selectExpiringMediaByUserCount: " + err.Error())
	}

	return stmts, nil
}

func (s *expiringMediaTableStatements) Prepare(ctx rcontext.RequestContext) *expiringMediaTableWithContext {
	return &expiringMediaTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *expiringMediaTableWithContext) Insert(origin string, mediaId string, userId string, expiresTs int64) error {
	_, err := s.statements.insertExpiringMedia.ExecContext(s.ctx, origin, mediaId, userId, expiresTs)
	return err
}

func (s *expiringMediaTableWithContext) ByUserCount(userId string) (int64, error) {
	row := s.statements.selectExpiringMediaByUserCount.QueryRowContext(s.ctx, userId, util.NowMillis())
	val := int64(0)
	err := row.Scan(&val)
	if err == sql.ErrNoRows {
		err = nil
		val = 0
	}
	return val, err
}
