package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type DbExpiringMedia struct {
	Origin    string
	MediaId   string
	UserId    string
	ExpiresTs int64
}

func (r *DbExpiringMedia) IsExpired() bool {
	expiresTs := time.UnixMilli(r.ExpiresTs)
	return expiresTs.Before(time.Now())
}

const insertExpiringMedia = "INSERT INTO expiring_media (origin, media_id, user_id, expires_ts) VALUES ($1, $2, $3, $4);"
const selectExpiringMediaByUserCount = "SELECT COUNT(*) FROM expiring_media WHERE user_id = $1 AND expires_ts >= $2;"
const selectExpiringMediaById = "SELECT origin, media_id, user_id, expires_ts FROM expiring_media WHERE origin = $1 AND media_id = $2;"
const deleteExpiringMediaById = "DELETE FROM expiring_media WHERE origin = $1 AND media_id = $2;"

// Dev note: there is an UPDATE query in the Upload test suite.

type expiringMediaTableStatements struct {
	insertExpiringMedia            *sql.Stmt
	selectExpiringMediaByUserCount *sql.Stmt
	selectExpiringMediaById        *sql.Stmt
	deleteExpiringMediaById        *sql.Stmt
}

type expiringMediaTableWithContext struct {
	statements *expiringMediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareExpiringMediaTables(db *sql.DB) (*expiringMediaTableStatements, error) {
	var err error
	stmts := &expiringMediaTableStatements{}

	if stmts.insertExpiringMedia, err = db.Prepare(insertExpiringMedia); err != nil {
		return nil, fmt.Errorf("error preparing insertExpiringMedia: %w", err)
	}
	if stmts.selectExpiringMediaByUserCount, err = db.Prepare(selectExpiringMediaByUserCount); err != nil {
		return nil, fmt.Errorf("error preparing selectExpiringMediaByUserCount: %w", err)
	}
	if stmts.selectExpiringMediaById, err = db.Prepare(selectExpiringMediaById); err != nil {
		return nil, fmt.Errorf("error preparing selectExpiringMediaById: %w", err)
	}
	if stmts.deleteExpiringMediaById, err = db.Prepare(deleteExpiringMediaById); err != nil {
		return nil, fmt.Errorf("error preparing deleteExpiringMediaById: %w", err)
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
	row := s.statements.selectExpiringMediaByUserCount.QueryRowContext(s.ctx, userId, time.Now().UnixMilli())
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
