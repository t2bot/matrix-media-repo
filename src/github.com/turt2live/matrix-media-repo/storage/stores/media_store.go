package stores

import (
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
)

const selectMedia = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, location, creation_ts FROM media WHERE origin = $1 and media_id = $2;"
const selectMediaByHash = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, location, creation_ts FROM media WHERE sha256_hash = $1;"
const insertMedia = "INSERT INTO media (origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, location, creation_ts) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);"

type mediaStoreStatements struct {
	selectMedia       *sql.Stmt
	selectMediaByHash *sql.Stmt
	insertMedia       *sql.Stmt
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
