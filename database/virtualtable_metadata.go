package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type VirtLastAccess struct {
	*Locatable
	SizeBytes    int64
	CreationTs   int64
	LastAccessTs int64
	ContentType  string
}

const selectEstimatedDatastoreSize = "SELECT COALESCE(SUM(m2.size_bytes), 0) + COALESCE((SELECT SUM(t2.size_bytes) FROM (SELECT DISTINCT t.sha256_hash, MAX(t.size_bytes) AS size_bytes FROM thumbnails AS t WHERE t.datastore_id = $1 GROUP BY t.sha256_hash) AS t2), 0) AS size_total FROM (SELECT DISTINCT m.sha256_hash, MAX(m.size_bytes) AS size_bytes FROM media AS m WHERE m.datastore_id = $1 GROUP BY m.sha256_hash) AS m2;"
const selectUploadSizesForServer = "SELECT COALESCE((SELECT SUM(size_bytes) FROM media WHERE origin = $1), 0) AS media, COALESCE((SELECT SUM(size_bytes) FROM thumbnails WHERE origin = $1), 0) AS thumbnails;"
const selectUploadCountsForServer = "SELECT COALESCE((SELECT COUNT(origin) FROM media WHERE origin = $1), 0) AS media, COALESCE((SELECT COUNT(origin) FROM thumbnails WHERE origin = $1), 0) AS thumbnails;"
const selectMediaForDatastoreWithLastAccess = "SELECT m.sha256_hash, m.size_bytes, m.datastore_id, m.location, m.creation_ts, a.last_access_ts, m.content_type FROM media AS m JOIN last_access AS a ON m.sha256_hash = a.sha256_hash WHERE a.last_access_ts < $1 AND m.datastore_id = $2;"
const selectThumbnailsForDatastoreWithLastAccess = "SELECT m.sha256_hash, m.size_bytes, m.datastore_id, m.location, m.creation_ts, a.last_access_ts, m.content_type FROM thumbnails AS m JOIN last_access AS a ON m.sha256_hash = a.sha256_hash WHERE a.last_access_ts < $1 AND m.datastore_id = $2;"
const updateQuarantineByHash = "WITH t AS (SELECT m.origin AS origin, m.media_id AS media_id, a.purpose AS purpose FROM media AS m LEFT JOIN media_attributes AS a ON m.origin = a.origin AND m.media_id = a.media_id WHERE m.sha256_hash = $1 AND (a.purpose IS NULL OR a.purpose <> $2) AND m.quarantined <> $3) UPDATE media AS m2 SET quarantined = $3 FROM t WHERE m2.origin = t.origin AND m2.media_id = t.media_id;"
const updateQuarantineByHashAndOrigin = "WITH t AS (SELECT m.origin AS origin, m.media_id AS media_id, a.purpose AS purpose FROM media AS m LEFT JOIN media_attributes AS a ON m.origin = a.origin AND m.media_id = a.media_id WHERE m.origin = $1 AND m.sha256_hash = $2 AND (a.purpose IS NULL OR a.purpose <> $3) AND m.quarantined <> $4) UPDATE media AS m2 SET quarantined = $4 FROM t WHERE m2.origin = t.origin AND m2.media_id = t.media_id;"

type SynStatUserOrderBy string

const (
	SynStatUserOrderByMediaCount  SynStatUserOrderBy = "media_count"
	SynStatUserOrderByMediaLength SynStatUserOrderBy = "media_length"
	SynStatUserOrderByUserId      SynStatUserOrderBy = "user_id"

	DefaultSynStatUserOrderBy = SynStatUserOrderByUserId
)

func IsSynStatUserOrderBy(orderBy SynStatUserOrderBy) bool {
	return orderBy == SynStatUserOrderByMediaCount || orderBy == SynStatUserOrderByMediaLength || orderBy == SynStatUserOrderByUserId
}

type DbSynUserStat struct {
	UserId      string
	MediaCount  int64
	MediaLength int64
}

type metadataVirtualTableStatements struct {
	db *sql.DB

	selectEstimatedDatastoreSize               *sql.Stmt
	selectUploadSizesForServer                 *sql.Stmt
	selectUploadCountsForServer                *sql.Stmt
	selectMediaForDatastoreWithLastAccess      *sql.Stmt
	selectThumbnailsForDatastoreWithLastAccess *sql.Stmt
	updateQuarantineByHash                     *sql.Stmt
	updateQuarantineByHashAndOrigin            *sql.Stmt
}

type metadataVirtualTableWithContext struct {
	statements *metadataVirtualTableStatements
	ctx        rcontext.RequestContext
}

func prepareMetadataVirtualTables(db *sql.DB) (*metadataVirtualTableStatements, error) {
	var err error
	var stmts = &metadataVirtualTableStatements{
		db: db,
	}

	if stmts.selectEstimatedDatastoreSize, err = db.Prepare(selectEstimatedDatastoreSize); err != nil {
		return nil, errors.New("error preparing selectEstimatedDatastoreSize: " + err.Error())
	}
	if stmts.selectUploadSizesForServer, err = db.Prepare(selectUploadSizesForServer); err != nil {
		return nil, errors.New("error preparing selectUploadSizesForServer: " + err.Error())
	}
	if stmts.selectUploadCountsForServer, err = db.Prepare(selectUploadCountsForServer); err != nil {
		return nil, errors.New("error preparing selectUploadCountsForServer: " + err.Error())
	}
	if stmts.selectMediaForDatastoreWithLastAccess, err = db.Prepare(selectMediaForDatastoreWithLastAccess); err != nil {
		return nil, errors.New("error preparing selectMediaForDatastoreWithLastAccess: " + err.Error())
	}
	if stmts.selectThumbnailsForDatastoreWithLastAccess, err = db.Prepare(selectThumbnailsForDatastoreWithLastAccess); err != nil {
		return nil, errors.New("error preparing selectThumbnailsForDatastoreWithLastAccess: " + err.Error())
	}
	if stmts.updateQuarantineByHash, err = db.Prepare(updateQuarantineByHash); err != nil {
		return nil, errors.New("error preparing updateQuarantineByHash: " + err.Error())
	}
	if stmts.updateQuarantineByHashAndOrigin, err = db.Prepare(updateQuarantineByHashAndOrigin); err != nil {
		return nil, errors.New("error preparing updateQuarantineByHashAndOrigin: " + err.Error())
	}

	return stmts, nil
}

func (s *metadataVirtualTableStatements) Prepare(ctx rcontext.RequestContext) *metadataVirtualTableWithContext {
	return &metadataVirtualTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *metadataVirtualTableWithContext) EstimateDatastoreSize(datastoreId string) (int64, error) {
	row := s.statements.selectEstimatedDatastoreSize.QueryRowContext(s.ctx, datastoreId)
	val := int64(0)
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = 0
	}
	return val, err
}

func (s *metadataVirtualTableWithContext) ByteUsageForServer(serverName string) (int64, int64, error) {
	row := s.statements.selectUploadSizesForServer.QueryRowContext(s.ctx, serverName)
	media := int64(0)
	thumbs := int64(0)
	err := row.Scan(&media, &thumbs)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		media = int64(0)
		thumbs = int64(0)
	}
	return media, thumbs, err
}

func (s *metadataVirtualTableWithContext) CountUsageForServer(serverName string) (int64, int64, error) {
	row := s.statements.selectUploadCountsForServer.QueryRowContext(s.ctx, serverName)
	media := int64(0)
	thumbs := int64(0)
	err := row.Scan(&media, &thumbs)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		media = int64(0)
		thumbs = int64(0)
	}
	return media, thumbs, err
}

func (s *metadataVirtualTableWithContext) UnoptimizedSynapseUserStatsPage(serverName string, orderBy SynStatUserOrderBy, startIdx int64, limit int64, fromTs int64, untilTs int64, search string, asc bool) ([]*DbSynUserStat, int64, error) {
	sqlDir := "DESC"
	if asc {
		sqlDir = "ASC"
	}

	if !IsSynStatUserOrderBy(orderBy) {
		return nil, 0, errors.New("sql injection prevented: orderBy must be recognized")
	}

	sqlParams := make([]interface{}, 0)
	sqlWhere := make([]string, 0)

	addParam := func(val interface{}) {
		sqlParams = append(sqlParams, val)
	}
	addWhere := func(str string, val interface{}) {
		addParam(val)
		sqlWhere = append(sqlWhere, fmt.Sprintf(str, len(sqlParams)))
	}

	addWhere("origin = $%d", serverName)
	if fromTs >= 0 {
		addWhere("creation_ts >= $%d", fromTs)
	}
	if untilTs >= 0 {
		addWhere("creation_ts <= $%d", untilTs)
	}
	if search != "" {
		addWhere("user_id LIKE $%d", fmt.Sprintf("@%%%s%%:%%", search))
	}

	addWhere("user_id <> $%d", "")

	addParam(limit)
	sqlLimit := fmt.Sprintf("LIMIT $%d", len(sqlParams))
	addParam(startIdx)
	sqlOffset := fmt.Sprintf("OFFSET $%d", len(sqlParams))

	sqlStart := fmt.Sprintf("FROM media WHERE %s GROUP BY user_id", strings.Join(sqlWhere, " AND "))

	sqlPageQ := fmt.Sprintf("SELECT COUNT(user_id) AS media_count, SUM(size_bytes) AS media_length, user_id %s ORDER BY %s %s %s %s;", sqlStart, orderBy, sqlDir, sqlLimit, sqlOffset)

	results := make([]*DbSynUserStat, 0)
	rows, err := s.statements.db.QueryContext(s.ctx, sqlPageQ, sqlParams...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return results, 0, nil
		}
		return nil, 0, err
	}
	for rows.Next() {
		val := &DbSynUserStat{}
		err = rows.Scan(&val.MediaCount, &val.MediaLength, &val.UserId)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, val)
	}

	sqlTotalQ := fmt.Sprintf("SELECT COUNT(*) FROM (SELECT user_id %s) AS count_user_ids;", sqlStart)
	sqlParams = sqlParams[:len(sqlParams)-2] // trim off LIMIT and OFFSET values
	row := s.statements.db.QueryRowContext(s.ctx, sqlTotalQ, sqlParams...)
	total := int64(0)
	err = row.Scan(&total)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*DbSynUserStat, 0), 0, nil
		}
		return nil, 0, err
	}

	return results, total, nil
}

func (s *metadataVirtualTableWithContext) scanLastAccess(rows *sql.Rows, err error) ([]*VirtLastAccess, error) {
	results := make([]*VirtLastAccess, 0)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return results, nil
		}
		return nil, err
	}
	for rows.Next() {
		val := &VirtLastAccess{Locatable: &Locatable{}}
		if err = rows.Scan(&val.Sha256Hash, &val.SizeBytes, &val.DatastoreId, &val.Location, &val.CreationTs, &val.LastAccessTs, &val.ContentType); err != nil {
			return nil, err
		}
		results = append(results, val)
	}

	return results, nil
}

func (s *metadataVirtualTableWithContext) GetMediaForDatastoreByLastAccess(datastoreId string, lastAccessTs int64) ([]*VirtLastAccess, error) {
	return s.scanLastAccess(s.statements.selectMediaForDatastoreWithLastAccess.QueryContext(s.ctx, lastAccessTs, datastoreId))
}

func (s *metadataVirtualTableWithContext) GetThumbnailsForDatastoreByLastAccess(datastoreId string, lastAccessTs int64) ([]*VirtLastAccess, error) {
	return s.scanLastAccess(s.statements.selectThumbnailsForDatastoreWithLastAccess.QueryContext(s.ctx, lastAccessTs, datastoreId))
}

func (s *metadataVirtualTableWithContext) UpdateQuarantineByHash(hash string, quarantined bool) (int64, error) {
	c, err := s.statements.updateQuarantineByHash.ExecContext(s.ctx, hash, PurposePinned, quarantined)
	if err != nil {
		return 0, err
	}
	return c.RowsAffected()
}

func (s *metadataVirtualTableWithContext) UpdateQuarantineByHashAndOrigin(origin string, hash string, quarantined bool) (int64, error) {
	c, err := s.statements.updateQuarantineByHashAndOrigin.ExecContext(s.ctx, origin, hash, PurposePinned, quarantined)
	if err != nil {
		return 0, err
	}
	return c.RowsAffected()
}
