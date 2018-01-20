package stores

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
)

const selectMedia = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, location, creation_ts FROM media WHERE origin = $1 and media_id = $2;"
const selectMediaByHash = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, location, creation_ts FROM media WHERE sha256_hash = $1;"
const insertMedia = "INSERT INTO media (origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, location, creation_ts) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);"
const selectOldMedia = "SELECT m.origin, m.media_id, m.upload_name, m.content_type, m.user_id, m.sha256_hash, m.size_bytes, m.location, m.creation_ts FROM media AS m WHERE NOT(m.origin = ANY($1)) AND m.creation_ts < $2 AND (SELECT COUNT(*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.creation_ts >= $2) = 0 AND (SELECT COUNT(*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.origin = ANY($1)) = 0;"
const selectOrigins = "SELECT DISTINCT origin FROM media;"
const deleteMedia = "DELETE FROM media WHERE origin = $1 AND media_id = $2;"

type mediaStoreStatements struct {
	selectMedia       *sql.Stmt
	selectMediaByHash *sql.Stmt
	insertMedia       *sql.Stmt
	selectOldMedia    *sql.Stmt
	selectOrigins     *sql.Stmt
	deleteMedia       *sql.Stmt
}

type MediaStoreFactory struct {
	sqlDb *sql.DB
	stmts *mediaStoreStatements
}

type MediaStore struct {
	factory    *MediaStoreFactory // just for reference
	ctx        context.Context
	log        *logrus.Entry
	statements *mediaStoreStatements // copied from factory
}

func InitMediaStore(sqlDb *sql.DB) (*MediaStoreFactory, error) {
	store := MediaStoreFactory{stmts: &mediaStoreStatements{}}
	var err error

	store.sqlDb = sqlDb

	if store.stmts.selectMedia, err = store.sqlDb.Prepare(selectMedia); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaByHash, err = store.sqlDb.Prepare(selectMediaByHash); err != nil {
		return nil, err
	}
	if store.stmts.insertMedia, err = store.sqlDb.Prepare(insertMedia); err != nil {
		return nil, err
	}
	if store.stmts.selectOldMedia, err = store.sqlDb.Prepare(selectOldMedia); err != nil {
		return nil, err
	}
	if store.stmts.selectOrigins, err = store.sqlDb.Prepare(selectOrigins); err != nil {
		return nil, err
	}
	if store.stmts.deleteMedia, err = store.sqlDb.Prepare(deleteMedia); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *MediaStoreFactory) Create(ctx context.Context, entry *logrus.Entry) (*MediaStore) {
	return &MediaStore{
		factory:    f,
		ctx:        ctx,
		log:        entry,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *MediaStore) Insert(media *types.Media) (error) {
	_, err := s.statements.insertMedia.ExecContext(
		s.ctx,
		media.Origin,
		media.MediaId,
		media.UploadName,
		media.ContentType,
		media.UserId,
		media.Sha256Hash,
		media.SizeBytes,
		media.Location,
		media.CreationTs,
	)
	return err
}

func (s *MediaStore) GetByHash(hash string) ([]*types.Media, error) {
	rows, err := s.statements.selectMediaByHash.QueryContext(s.ctx, hash)
	if err != nil {
		return nil, err
	}

	var results []*types.Media
	for rows.Next() {
		obj := &types.Media{}
		err = rows.Scan(
			&obj.Origin,
			&obj.MediaId,
			&obj.UploadName,
			&obj.ContentType,
			&obj.UserId,
			&obj.Sha256Hash,
			&obj.SizeBytes,
			&obj.Location,
			&obj.CreationTs,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *MediaStore) Get(origin string, mediaId string) (*types.Media, error) {
	m := &types.Media{}
	err := s.statements.selectMedia.QueryRowContext(s.ctx, origin, mediaId).Scan(
		&m.Origin,
		&m.MediaId,
		&m.UploadName,
		&m.ContentType,
		&m.UserId,
		&m.Sha256Hash,
		&m.SizeBytes,
		&m.Location,
		&m.CreationTs,
	)
	return m, err
}

func (s *MediaStore) GetOldMedia(exceptOrigins []string, beforeTs int64) ([]*types.Media, error) {
	rows, err := s.statements.selectOldMedia.QueryContext(s.ctx, pq.Array(exceptOrigins), beforeTs)
	if err != nil {
		return nil, err
	}

	var results []*types.Media
	for rows.Next() {
		obj := &types.Media{}
		err = rows.Scan(
			&obj.Origin,
			&obj.MediaId,
			&obj.UploadName,
			&obj.ContentType,
			&obj.UserId,
			&obj.Sha256Hash,
			&obj.SizeBytes,
			&obj.Location,
			&obj.CreationTs,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *MediaStore) GetOrigins() ([]string, error) {
	rows, err := s.statements.selectOrigins.QueryContext(s.ctx)
	if err != nil {
		return nil, err
	}

	var results []string
	for rows.Next() {
		obj := ""
		err = rows.Scan(&obj)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *MediaStore) Delete(origin string, mediaId string) (error) {
	_, err := s.statements.deleteMedia.ExecContext(s.ctx, origin, mediaId)
	return err
}
