package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type Locatable struct {
	Sha256Hash  string
	DatastoreId string
	Location    string
}

type DbMedia struct {
	*Locatable
	Origin      string
	MediaId     string
	UploadName  string
	ContentType string
	UserId      string
	//Sha256Hash  string
	SizeBytes   int64
	CreationTs  int64
	Quarantined bool
	//DatastoreId string
	//Location    string
}

const selectDistinctMediaDatastoreIds = "SELECT DISTINCT datastore_id FROM media;"
const selectMediaIsQuarantinedByHash = "SELECT quarantined FROM media WHERE quarantined = TRUE AND sha256_hash = $1;"
const selectMediaByHash = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE sha256_hash = $1;"
const insertMedia = "INSERT INTO media (origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);"
const selectMediaExists = "SELECT TRUE FROM media WHERE origin = $1 AND media_id = $2 LIMIT 1;"
const selectMediaById = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE origin = $1 AND media_id = $2;"
const selectMediaByUserId = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE user_id = $1;"
const selectOldMediaByUserId = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE user_id = $1 AND creation_ts < $2;"
const selectMediaByOrigin = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE origin = $1;"
const selectOldMediaByOrigin = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE origin = $1 AND creation_ts < $2;"
const selectMediaByLocationExists = "SELECT TRUE FROM media WHERE datastore_id = $1 AND location = $2 LIMIT 1;"
const selectMediaByUserCount = "SELECT COUNT(*) FROM media WHERE user_id = $1;"
const selectMediaByOriginAndUserIds = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE origin = $1 AND user_id = ANY($2);"
const selectMediaByOriginAndIds = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE origin = $1 AND media_id = ANY($2);"
const selectOldMediaExcludingDomains = "SELECT m.origin, m.media_id, m.upload_name, m.content_type, m.user_id, m.sha256_hash, m.size_bytes, m.creation_ts, m.quarantined, m.datastore_id, m.location FROM media AS m WHERE m.origin <> ANY($1) AND m.creation_ts < $2 AND (SELECT COUNT(d.*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.creation_ts >= $2) = 0 AND (SELECT COUNT(d.*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.origin = ANY($1)) = 0;"
const deleteMedia = "DELETE FROM media WHERE origin = $1 AND media_id = $2;"
const updateMediaLocation = "UPDATE media SET datastore_id = $3, location = $4 WHERE datastore_id = $1 AND location = $2;"
const selectMediaByLocation = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE datastore_id = $1 AND location = $2;"
const selectMediaByQuarantine = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE quarantined = TRUE;"
const selectMediaByQuarantineAndOrigin = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE quarantined = TRUE AND origin = $1;"

type mediaTableStatements struct {
	selectDistinctMediaDatastoreIds  *sql.Stmt
	selectMediaIsQuarantinedByHash   *sql.Stmt
	selectMediaByHash                *sql.Stmt
	insertMedia                      *sql.Stmt
	selectMediaExists                *sql.Stmt
	selectMediaById                  *sql.Stmt
	selectMediaByUserId              *sql.Stmt
	selectOldMediaByUserId           *sql.Stmt
	selectMediaByOrigin              *sql.Stmt
	selectOldMediaByOrigin           *sql.Stmt
	selectMediaByLocationExists      *sql.Stmt
	selectMediaByUserCount           *sql.Stmt
	selectMediaByOriginAndUserIds    *sql.Stmt
	selectMediaByOriginAndIds        *sql.Stmt
	selectOldMediaExcludingDomains   *sql.Stmt
	deleteMedia                      *sql.Stmt
	updateMediaLocation              *sql.Stmt
	selectMediaByLocation            *sql.Stmt
	selectMediaByQuarantine          *sql.Stmt
	selectMediaByQuarantineAndOrigin *sql.Stmt
}

type MediaTableWithContext struct {
	statements *mediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareMediaTables(db *sql.DB) (*mediaTableStatements, error) {
	var err error
	stmts := &mediaTableStatements{}

	if stmts.selectDistinctMediaDatastoreIds, err = db.Prepare(selectDistinctMediaDatastoreIds); err != nil {
		return nil, fmt.Errorf("error preparing selectDistinctMediaDatastoreIds: %w", err)
	}
	if stmts.selectMediaIsQuarantinedByHash, err = db.Prepare(selectMediaIsQuarantinedByHash); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaIsQuarantinedByHash: %w", err)
	}
	if stmts.selectMediaByHash, err = db.Prepare(selectMediaByHash); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByHash: %w", err)
	}
	if stmts.insertMedia, err = db.Prepare(insertMedia); err != nil {
		return nil, fmt.Errorf("error preparing insertMedia: %w", err)
	}
	if stmts.selectMediaExists, err = db.Prepare(selectMediaExists); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaExists: %w", err)
	}
	if stmts.selectMediaById, err = db.Prepare(selectMediaById); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaById: %w", err)
	}
	if stmts.selectMediaByUserId, err = db.Prepare(selectMediaByUserId); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByUserId: %w", err)
	}
	if stmts.selectOldMediaByUserId, err = db.Prepare(selectOldMediaByUserId); err != nil {
		return nil, fmt.Errorf("error preparing selectOldMediaByUserId: %w", err)
	}
	if stmts.selectMediaByOrigin, err = db.Prepare(selectMediaByOrigin); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByOrigin: %w", err)
	}
	if stmts.selectOldMediaByOrigin, err = db.Prepare(selectOldMediaByOrigin); err != nil {
		return nil, fmt.Errorf("error preparing selectOldMediaByOrigin: %w", err)
	}
	if stmts.selectMediaByLocationExists, err = db.Prepare(selectMediaByLocationExists); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByLocationExists: %w", err)
	}
	if stmts.selectMediaByUserCount, err = db.Prepare(selectMediaByUserCount); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByUserCount: %w", err)
	}
	if stmts.selectMediaByOriginAndUserIds, err = db.Prepare(selectMediaByOriginAndUserIds); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByOriginAndUserIds: %w", err)
	}
	if stmts.selectMediaByOriginAndIds, err = db.Prepare(selectMediaByOriginAndIds); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByOriginAndIds: %w", err)
	}
	if stmts.selectOldMediaExcludingDomains, err = db.Prepare(selectOldMediaExcludingDomains); err != nil {
		return nil, fmt.Errorf("error preparing selectOldMediaExcludingDomains: %w", err)
	}
	if stmts.deleteMedia, err = db.Prepare(deleteMedia); err != nil {
		return nil, fmt.Errorf("error preparing deleteMedia: %w", err)
	}
	if stmts.updateMediaLocation, err = db.Prepare(updateMediaLocation); err != nil {
		return nil, fmt.Errorf("error preparing updateMediaLocation: %w", err)
	}
	if stmts.selectMediaByLocation, err = db.Prepare(selectMediaByLocation); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByLocation: %w", err)
	}
	if stmts.selectMediaByQuarantine, err = db.Prepare(selectMediaByQuarantine); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByQuarantine: %w", err)
	}
	if stmts.selectMediaByQuarantineAndOrigin, err = db.Prepare(selectMediaByQuarantineAndOrigin); err != nil {
		return nil, fmt.Errorf("error preparing selectMediaByQuarantineAndOrigin: %w", err)
	}

	return stmts, nil
}

func (s *mediaTableStatements) Prepare(ctx rcontext.RequestContext) *MediaTableWithContext {
	return &MediaTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *MediaTableWithContext) GetDistinctDatastoreIds() ([]string, error) {
	results := make([]string, 0)
	rows, err := s.statements.selectDistinctMediaDatastoreIds.QueryContext(s.ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return results, nil
		}
		return nil, err
	}

	for rows.Next() {
		val := ""
		if err = rows.Scan(&val); err != nil {
			return nil, err
		}
		results = append(results, val)
	}

	return results, nil
}

func (s *MediaTableWithContext) IsHashQuarantined(sha256hash string) (bool, error) {
	// TODO: https://github.com/t2bot/matrix-media-repo/issues/410
	row := s.statements.selectMediaIsQuarantinedByHash.QueryRowContext(s.ctx, sha256hash)
	val := false
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = false
	}
	return val, err
}

func (s *MediaTableWithContext) scanRows(rows *sql.Rows, err error) ([]*DbMedia, error) {
	results := make([]*DbMedia, 0)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return results, nil
		}
		return nil, err
	}
	for rows.Next() {
		val := &DbMedia{Locatable: &Locatable{}}
		if err = rows.Scan(&val.Origin, &val.MediaId, &val.UploadName, &val.ContentType, &val.UserId, &val.Sha256Hash, &val.SizeBytes, &val.CreationTs, &val.Quarantined, &val.DatastoreId, &val.Location); err != nil {
			return nil, err
		}
		results = append(results, val)
	}

	return results, nil
}

func (s *MediaTableWithContext) GetByHash(sha256hash string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByHash.QueryContext(s.ctx, sha256hash))
}

func (s *MediaTableWithContext) GetByUserId(userId string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByUserId.QueryContext(s.ctx, userId))
}

func (s *MediaTableWithContext) GetOldByUserId(userId string, beforeTs int64) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectOldMediaByUserId.QueryContext(s.ctx, userId, beforeTs))
}

func (s *MediaTableWithContext) GetByOrigin(origin string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByOrigin.QueryContext(s.ctx, origin))
}

func (s *MediaTableWithContext) GetOldByOrigin(origin string, beforeTs int64) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectOldMediaByOrigin.QueryContext(s.ctx, origin, beforeTs))
}

func (s *MediaTableWithContext) GetByOriginUsers(origin string, userIds []string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByOriginAndUserIds.QueryContext(s.ctx, origin, pq.Array(userIds)))
}

func (s *MediaTableWithContext) GetByIds(origin string, mediaIds []string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByOriginAndIds.QueryContext(s.ctx, origin, pq.Array(mediaIds)))
}

func (s *MediaTableWithContext) GetOldExcluding(origins []string, beforeTs int64) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectOldMediaExcludingDomains.QueryContext(s.ctx, pq.Array(origins), beforeTs))
}

func (s *MediaTableWithContext) GetByLocation(datastoreId string, location string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByLocation.QueryContext(s.ctx, datastoreId, location))
}

func (s *MediaTableWithContext) GetByQuarantine() ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByQuarantine.QueryContext(s.ctx))
}

func (s *MediaTableWithContext) GetByOriginQuarantine(origin string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByQuarantineAndOrigin.QueryContext(s.ctx, origin))
}

func (s *MediaTableWithContext) GetById(origin string, mediaId string) (*DbMedia, error) {
	row := s.statements.selectMediaById.QueryRowContext(s.ctx, origin, mediaId)
	val := &DbMedia{Locatable: &Locatable{}}
	err := row.Scan(&val.Origin, &val.MediaId, &val.UploadName, &val.ContentType, &val.UserId, &val.Sha256Hash, &val.SizeBytes, &val.CreationTs, &val.Quarantined, &val.DatastoreId, &val.Location)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = nil
	}
	return val, err
}

func (s *MediaTableWithContext) ByUserCount(userId string) (int64, error) {
	row := s.statements.selectMediaByUserCount.QueryRowContext(s.ctx, userId)
	val := int64(0)
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = 0
	}
	return val, err
}

func (s *MediaTableWithContext) IdExists(origin string, mediaId string) (bool, error) {
	row := s.statements.selectMediaExists.QueryRowContext(s.ctx, origin, mediaId)
	val := false
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = false
	}
	return val, err
}

func (s *MediaTableWithContext) LocationExists(datastoreId string, location string) (bool, error) {
	row := s.statements.selectMediaByLocationExists.QueryRowContext(s.ctx, datastoreId, location)
	val := false
	err := row.Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = false
	}
	return val, err
}

func (s *MediaTableWithContext) Insert(record *DbMedia) error {
	_, err := s.statements.insertMedia.ExecContext(s.ctx, record.Origin, record.MediaId, record.UploadName, record.ContentType, record.UserId, record.Sha256Hash, record.SizeBytes, record.CreationTs, record.Quarantined, record.DatastoreId, record.Location)
	return err
}

func (s *MediaTableWithContext) Delete(origin string, mediaId string) error {
	_, err := s.statements.deleteMedia.ExecContext(s.ctx, origin, mediaId)
	return err
}

func (s *MediaTableWithContext) UpdateLocation(sourceDsId string, sourceLocation string, targetDsId string, targetLocation string) error {
	_, err := s.statements.updateMediaLocation.ExecContext(s.ctx, sourceDsId, sourceLocation, targetDsId, targetLocation)
	return err
}
