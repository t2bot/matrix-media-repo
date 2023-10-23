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

func (r *DbExpiringMedia) IsExpired() bool {
	return r.ExpiresTs < util.NowMillis()
}

const insertExpiringMedia = "INSERT INTO expiring_media (origin, media_id, user_id, expires_ts) VALUES ($1, $2, $3, $4);"
const selectExpiringMediaByUserCount = "SELECT COUNT(*) FROM expiring_media WHERE user_id = $1 AND expires_ts >= $2;"
const selectExpiringMediaById = "SELECT origin, media_id, user_id, expires_ts FROM expiring_media WHERE origin = $1 AND media_id = $2;"
const deleteExpiringMediaById = "DELETE FROM expiring_media WHERE origin = $1 AND media_id = $2;"
const updateExpiringMediaExpiration = "UPDATE expiring_media SET expires_ts = $3 WHERE origin = $1 AND media_id = $2;"

// Dev note: there is an UPDATE query in the Upload test suite.

type expiringMediaTableStatements struct {
	insertExpiringMedia            *sql.Stmt
	selectExpiringMediaByUserCount *sql.Stmt
	selectExpiringMediaById        *sql.Stmt
	deleteExpiringMediaById        *sql.Stmt
	updateExpiringMediaExpiration  *sql.Stmt
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
	if stmts.selectExpiringMediaById, err = db.Prepare(selectExpiringMediaById); err != nil {
		return nil, errors.New("error preparing selectExpiringMediaById: " + err.Error())
	}
	if stmts.deleteExpiringMediaById, err = db.Prepare(deleteExpiringMediaById); err != nil {
		return nil, errors.New("error preparing deleteExpiringMediaById: " + err.Error())
	}
	if stmts.updateExpiringMediaExpiration, err = db.Prepare(updateExpiringMediaExpiration); err != nil {
		return nil, errors.New("error preparing updateExpiringMediaExpiration: " + err.Error())
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
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = 0
	}
	return val, err
}

func (s *expiringMediaTableWithContext) Get(origin string, mediaId string) (*DbExpiringMedia, error) {
	row := s.statements.selectExpiringMediaById.QueryRowContext(s.ctx, origin, mediaId)
	val := &DbExpiringMedia{}
	err := row.Scan(&val.Origin, &val.MediaId, &val.UserId, &val.ExpiresTs)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = nil
	}
	return val, err
}

func (s *expiringMediaTableWithContext) Delete(origin string, mediaId string) error {
	_, err := s.statements.deleteExpiringMediaById.ExecContext(s.ctx, origin, mediaId)
	return err
}

func (s *expiringMediaTableWithContext) SetExpiry(origin string, mediaId string, expiresTs int64) error {
	_, err := s.statements.updateExpiringMediaExpiration.ExecContext(s.ctx, origin, mediaId, expiresTs)
	return err
}
