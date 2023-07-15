package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
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
const selectMediaByOrigin = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, creation_ts, quarantined, datastore_id, location FROM media WHERE origin = $1;"

type mediaTableStatements struct {
	selectDistinctMediaDatastoreIds *sql.Stmt
	selectMediaIsQuarantinedByHash  *sql.Stmt
	selectMediaByHash               *sql.Stmt
	insertMedia                     *sql.Stmt
	selectMediaExists               *sql.Stmt
	selectMediaById                 *sql.Stmt
	selectMediaByUserId             *sql.Stmt
	selectMediaByOrigin             *sql.Stmt
}

type mediaTableWithContext struct {
	statements *mediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareMediaTables(db *sql.DB) (*mediaTableStatements, error) {
	var err error
	var stmts = &mediaTableStatements{}

	if stmts.selectDistinctMediaDatastoreIds, err = db.Prepare(selectDistinctMediaDatastoreIds); err != nil {
		return nil, errors.New("error preparing selectDistinctMediaDatastoreIds: " + err.Error())
	}
	if stmts.selectMediaIsQuarantinedByHash, err = db.Prepare(selectMediaIsQuarantinedByHash); err != nil {
		return nil, errors.New("error preparing selectMediaIsQuarantinedByHash: " + err.Error())
	}
	if stmts.selectMediaByHash, err = db.Prepare(selectMediaByHash); err != nil {
		return nil, errors.New("error preparing selectMediaByHash: " + err.Error())
	}
	if stmts.insertMedia, err = db.Prepare(insertMedia); err != nil {
		return nil, errors.New("error preparing insertMedia: " + err.Error())
	}
	if stmts.selectMediaExists, err = db.Prepare(selectMediaExists); err != nil {
		return nil, errors.New("error preparing selectMediaExists: " + err.Error())
	}
	if stmts.selectMediaById, err = db.Prepare(selectMediaById); err != nil {
		return nil, errors.New("error preparing selectMediaById: " + err.Error())
	}
	if stmts.selectMediaByUserId, err = db.Prepare(selectMediaByUserId); err != nil {
		return nil, errors.New("error preparing selectMediaByUserId: " + err.Error())
	}
	if stmts.selectMediaByOrigin, err = db.Prepare(selectMediaByOrigin); err != nil {
		return nil, errors.New("error preparing selectMediaByOrigin: " + err.Error())
	}

	return stmts, nil
}

func (s *mediaTableStatements) Prepare(ctx rcontext.RequestContext) *mediaTableWithContext {
	return &mediaTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *mediaTableWithContext) GetDistinctDatastoreIds() ([]string, error) {
	results := make([]string, 0)
	rows, err := s.statements.selectDistinctMediaDatastoreIds.QueryContext(s.ctx)
	if err != nil {
		if err == sql.ErrNoRows {
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

func (s *mediaTableWithContext) IsHashQuarantined(sha256hash string) (bool, error) {
	// TODO: https://github.com/turt2live/matrix-media-repo/issues/410
	row := s.statements.selectMediaIsQuarantinedByHash.QueryRowContext(s.ctx, sha256hash)
	val := false
	err := row.Scan(&val)
	if err == sql.ErrNoRows {
		err = nil
		val = false
	}
	return val, err
}

func (s *mediaTableWithContext) scanRows(rows *sql.Rows, err error) ([]*DbMedia, error) {
	results := make([]*DbMedia, 0)
	if err != nil {
		if err == sql.ErrNoRows {
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

func (s *mediaTableWithContext) GetByHash(sha256hash string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByHash.QueryContext(s.ctx, sha256hash))
}

func (s *mediaTableWithContext) GetByUserId(userId string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByUserId.QueryContext(s.ctx, userId))
}

func (s *mediaTableWithContext) GetByOrigin(origin string) ([]*DbMedia, error) {
	return s.scanRows(s.statements.selectMediaByOrigin.QueryContext(s.ctx, origin))
}

func (s *mediaTableWithContext) GetById(origin string, mediaId string) (*DbMedia, error) {
	row := s.statements.selectMediaById.QueryRowContext(s.ctx, origin, mediaId)
	val := &DbMedia{Locatable: &Locatable{}}
	err := row.Scan(&val.Origin, &val.MediaId, &val.UploadName, &val.ContentType, &val.UserId, &val.Sha256Hash, &val.SizeBytes, &val.CreationTs, &val.Quarantined, &val.DatastoreId, &val.Location)
	if err == sql.ErrNoRows {
		err = nil
		val = nil
	}
	return val, err
}

func (s *mediaTableWithContext) IdExists(origin string, mediaId string) (bool, error) {
	row := s.statements.selectMediaExists.QueryRowContext(s.ctx, origin, mediaId)
	val := false
	err := row.Scan(&val)
	if err == sql.ErrNoRows {
		err = nil
		val = false
	}
	return val, err
}

func (s *mediaTableWithContext) Insert(record *DbMedia) error {
	_, err := s.statements.insertMedia.ExecContext(s.ctx, record.Origin, record.MediaId, record.UploadName, record.ContentType, record.UserId, record.Sha256Hash, record.SizeBytes, record.CreationTs, record.Quarantined, record.DatastoreId, record.Location)
	return err
}
