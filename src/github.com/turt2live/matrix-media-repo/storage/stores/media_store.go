package stores

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
)

const selectMedia = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE origin = $1 and media_id = $2;"
const selectMediaByHash = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE sha256_hash = $1;"
const insertMedia = "INSERT INTO media (origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);"
const selectOldMedia = "SELECT m.origin, m.media_id, m.upload_name, m.content_type, m.user_id, m.sha256_hash, m.size_bytes, m.datastore_id, m.location, m.creation_ts, quarantined FROM media AS m WHERE NOT(m.origin = ANY($1)) AND m.creation_ts < $2 AND (SELECT COUNT(*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.creation_ts >= $2) = 0 AND (SELECT COUNT(*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.origin = ANY($1)) = 0;"
const selectOrigins = "SELECT DISTINCT origin FROM media;"
const deleteMedia = "DELETE FROM media WHERE origin = $1 AND media_id = $2;"
const updateQuarantined = "UPDATE media SET quarantined = $3 WHERE origin = $1 AND media_id = $2;"
const selectDatastore = "SELECT datastore_id, ds_type, uri FROM datastores WHERE datastore_id = $1;"
const selectDatastoreByUri = "SELECT datastore_id, ds_type, uri FROM datastores WHERE uri = $1;"
const insertDatastore = "INSERT INTO datastores (datastore_id, ds_type, uri) VALUES ($1, $2, $3);"
const selectMediaWithoutDatastore = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE datastore_id IS NULL OR datastore_id = '';"
const updateMediaDatastoreAndLocation = "UPDATE media SET location = $4, datastore_id = $3 WHERE origin = $1 AND media_id = $2;"

type mediaStoreStatements struct {
	selectMedia                     *sql.Stmt
	selectMediaByHash               *sql.Stmt
	insertMedia                     *sql.Stmt
	selectOldMedia                  *sql.Stmt
	selectOrigins                   *sql.Stmt
	deleteMedia                     *sql.Stmt
	updateQuarantined               *sql.Stmt
	selectDatastore                 *sql.Stmt
	selectDatastoreByUri            *sql.Stmt
	insertDatastore                 *sql.Stmt
	selectMediaWithoutDatastore     *sql.Stmt
	updateMediaDatastoreAndLocation *sql.Stmt
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
	if store.stmts.updateQuarantined, err = store.sqlDb.Prepare(updateQuarantined); err != nil {
		return nil, err
	}
	if store.stmts.selectDatastore, err = store.sqlDb.Prepare(selectDatastore); err != nil {
		return nil, err
	}
	if store.stmts.selectDatastoreByUri, err = store.sqlDb.Prepare(selectDatastoreByUri); err != nil {
		return nil, err
	}
	if store.stmts.insertDatastore, err = store.sqlDb.Prepare(insertDatastore); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaWithoutDatastore, err = store.sqlDb.Prepare(selectMediaWithoutDatastore); err != nil {
		return nil, err
	}
	if store.stmts.updateMediaDatastoreAndLocation, err = store.sqlDb.Prepare(updateMediaDatastoreAndLocation); err != nil {
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
		media.DatastoreId,
		media.Location,
		media.CreationTs,
		media.Quarantined,
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
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.Quarantined,
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
		&m.DatastoreId,
		&m.Location,
		&m.CreationTs,
		&m.Quarantined,
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
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.Quarantined,
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

func (s *MediaStore) SetQuarantined(origin string, mediaId string, isQuarantined bool) (error) {
	_, err := s.statements.updateQuarantined.ExecContext(s.ctx, origin, mediaId, isQuarantined)
	return err
}

func (s *MediaStore) UpdateDatastoreAndLocation(media *types.Media) (error) {
	_, err := s.statements.updateMediaDatastoreAndLocation.ExecContext(
		s.ctx,
		media.Origin,
		media.MediaId,
		media.DatastoreId,
		media.Location,
	)

	return err
}

func (s *MediaStore) GetDatastore(id string) (*types.Datastore, error) {
	d := &types.Datastore{}
	err := s.statements.selectDatastore.QueryRowContext(s.ctx, id).Scan(
		&d.DatastoreId,
		&d.Type,
		&d.Uri,
	)
	return d, err
}

func (s *MediaStore) InsertDatastore(datastore *types.Datastore) (error) {
	_, err := s.statements.insertDatastore.ExecContext(
		s.ctx,
		datastore.DatastoreId,
		datastore.Type,
		datastore.Uri,
	)
	return err
}

func (s *MediaStore) GetDatastoreByUri(uri string) (*types.Datastore, error) {
	d := &types.Datastore{}
	err := s.statements.selectDatastoreByUri.QueryRowContext(s.ctx, uri).Scan(
		&d.DatastoreId,
		&d.Type,
		&d.Uri,
	)
	return d, err
}

func (s *MediaStore) GetAllWithoutDatastore() ([]*types.Media, error) {
	rows, err := s.statements.selectMediaWithoutDatastore.QueryContext(s.ctx)
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
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.Quarantined,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}
