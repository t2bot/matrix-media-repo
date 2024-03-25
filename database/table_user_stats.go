package database

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type UserQuota struct {
	MaxBytes   int64
	MaxPending int64
	MaxFiles   int64
}

type DbUserStats struct {
	UserId        string
	UploadedBytes int64
	UserQuota     *UserQuota
	// UserQuotaMaxBytes   int64
	// UserQuotaMaxPending int64
	// UserQuotaMaxFiles   int64
}

const selectUserStatsUploadedBytes = "SELECT uploaded_bytes FROM user_stats WHERE user_id = $1;"
const selectUserQuota = "SELECT user_id, uploaded_bytes, quota_max_bytes, quota_max_pending, quota_max_files FROM user_stats WHERE user_id = ANY($1);"
const updateUserQuota = "UPDATE user_stats SET quota_max_bytes = $2, quota_max_pending = $3, quota_max_files = $4 WHERE user_id = $1;"
const insertUserQuota = "INSERT INTO user_stats (user_id, uploaded_bytes, quota_max_bytes, quota_max_pending, quota_max_files) VALUES ($1, $2, $3, $4, $5);"

type userStatsTableStatements struct {
	selectUserStatsUploadedBytes *sql.Stmt
	selectUserQuota              *sql.Stmt
	updateUserQuota              *sql.Stmt
	insertUserQuota              *sql.Stmt
}

type userStatsTableWithContext struct {
	statements *userStatsTableStatements
	ctx        rcontext.RequestContext
}

func prepareUserStatsTables(db *sql.DB) (*userStatsTableStatements, error) {
	var err error
	var stmts = &userStatsTableStatements{}

	if stmts.selectUserStatsUploadedBytes, err = db.Prepare(selectUserStatsUploadedBytes); err != nil {
		return nil, errors.New("error preparing selectUserStatsUploadedBytes: " + err.Error())
	}
	if stmts.selectUserQuota, err = db.Prepare(selectUserQuota); err != nil {
		return nil, errors.New("error preparing selectUserQuota: " + err.Error())
	}
	if stmts.updateUserQuota, err = db.Prepare(updateUserQuota); err != nil {
		return nil, errors.New("error preparing updateUserQuota: " + err.Error())
	}
	if stmts.insertUserQuota, err = db.Prepare(insertUserQuota); err != nil {
		return nil, errors.New("error preparing insertUserQuota: " + err.Error())
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

func (s *userStatsTableWithContext) GetUserQuota(userIds []string) ([]*DbUserStats, error) {
	rows, err := s.statements.selectUserQuota.QueryContext(s.ctx, pq.Array(userIds))

	results := make([]*DbUserStats, 0)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return results, nil
		}
		return nil, err
	}
	for rows.Next() {
		val := &DbUserStats{UserQuota: &UserQuota{}}
		if err = rows.Scan(&val.UserId, &val.UploadedBytes, &val.UserQuota.MaxBytes, &val.UserQuota.MaxPending, &val.UserQuota.MaxFiles); err != nil {
			return nil, err
		}
		results = append(results, val)
	}

	return results, nil
}

func (s *userStatsTableWithContext) SetUserQuota(userId string, maxBytes int64, maxPending int64, maxFiles int64) error {
	// Need to insert default record if user has not uploaded any media beforehand
	row := s.statements.selectUserQuota.QueryRowContext(s.ctx, pq.Array([]string{userId}))
	val := &DbUserStats{UserQuota: &UserQuota{}}
	err := row.Scan(&val.UserId, &val.UploadedBytes, &val.UserQuota.MaxBytes, &val.UserQuota.MaxPending, &val.UserQuota.MaxFiles)

	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.statements.insertUserQuota.ExecContext(s.ctx, userId, 0, maxBytes, maxFiles, maxPending)
	} else {
		_, err = s.statements.updateUserQuota.ExecContext(s.ctx, userId, maxBytes, maxFiles, maxPending)
	}

	return err
}
