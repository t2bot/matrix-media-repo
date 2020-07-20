package stores

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
)

const selectMedia = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE origin = $1 and media_id = $2;"
const selectMediaByHash = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE sha256_hash = $1;"
const insertMedia = "INSERT INTO media (origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);"
const selectOldMedia = "SELECT m.origin, m.media_id, m.upload_name, m.content_type, m.user_id, m.sha256_hash, m.size_bytes, m.datastore_id, m.location, m.creation_ts, quarantined FROM media AS m WHERE m.origin <> ANY($1) AND m.creation_ts < $2 AND (SELECT COUNT(*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.creation_ts >= $2) = 0 AND (SELECT COUNT(*) FROM media AS d WHERE d.sha256_hash = m.sha256_hash AND d.origin = ANY($1)) = 0;"
const selectOrigins = "SELECT DISTINCT origin FROM media;"
const deleteMedia = "DELETE FROM media WHERE origin = $1 AND media_id = $2;"
const updateQuarantined = "UPDATE media SET quarantined = $3 WHERE origin = $1 AND media_id = $2;"
const selectDatastore = "SELECT datastore_id, ds_type, uri FROM datastores WHERE datastore_id = $1;"
const selectDatastoreByUri = "SELECT datastore_id, ds_type, uri FROM datastores WHERE uri = $1;"
const insertDatastore = "INSERT INTO datastores (datastore_id, ds_type, uri) VALUES ($1, $2, $3);"
const selectMediaWithoutDatastore = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE datastore_id IS NULL OR datastore_id = '';"
const updateMediaDatastoreAndLocation = "UPDATE media SET location = $4, datastore_id = $3 WHERE origin = $1 AND media_id = $2;"
const selectAllDatastores = "SELECT datastore_id, ds_type, uri FROM datastores;"
const selectAllMediaForServer = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE origin = $1"
const selectAllMediaForServerUsers = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE origin = $1 AND user_id = ANY($2)"
const selectAllMediaForServerIds = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE origin = $1 AND media_id = ANY($2)"
const selectQuarantinedMedia = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE quarantined = true;"
const selectServerQuarantinedMedia = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE quarantined = true AND origin = $1;"
const selectMediaByUser = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE user_id = $1"
const selectMediaByUserBefore = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE user_id = $1 AND creation_ts <= $2"
const selectMediaByDomainBefore = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE origin = $1 AND creation_ts <= $2"
const selectMediaByLocation = "SELECT origin, media_id, upload_name, content_type, user_id, sha256_hash, size_bytes, datastore_id, location, creation_ts, quarantined FROM media WHERE datastore_id = $1 AND location = $2"
const selectIfQuarantined = "SELECT 1 FROM media WHERE sha256_hash = $1 AND quarantined = $2 LIMIT 1;"

var dsCacheByPath = sync.Map{} // [string] => Datastore
var dsCacheById = sync.Map{}   // [string] => Datastore

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
	selectAllDatastores             *sql.Stmt
	selectMediaInDatastoreOlderThan *sql.Stmt
	selectAllMediaForServer         *sql.Stmt
	selectAllMediaForServerUsers    *sql.Stmt
	selectAllMediaForServerIds      *sql.Stmt
	selectQuarantinedMedia          *sql.Stmt
	selectServerQuarantinedMedia    *sql.Stmt
	selectMediaByUser               *sql.Stmt
	selectMediaByUserBefore         *sql.Stmt
	selectMediaByDomainBefore       *sql.Stmt
	selectMediaByLocation           *sql.Stmt
	selectIfQuarantined             *sql.Stmt
}

type MediaStoreFactory struct {
	sqlDb *sql.DB
	stmts *mediaStoreStatements
}

type MediaStore struct {
	factory    *MediaStoreFactory // just for reference
	ctx        rcontext.RequestContext
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
	if store.stmts.selectAllDatastores, err = store.sqlDb.Prepare(selectAllDatastores); err != nil {
		return nil, err
	}
	if store.stmts.selectAllMediaForServer, err = store.sqlDb.Prepare(selectAllMediaForServer); err != nil {
		return nil, err
	}
	if store.stmts.selectAllMediaForServerUsers, err = store.sqlDb.Prepare(selectAllMediaForServerUsers); err != nil {
		return nil, err
	}
	if store.stmts.selectAllMediaForServerIds, err = store.sqlDb.Prepare(selectAllMediaForServerIds); err != nil {
		return nil, err
	}
	if store.stmts.selectQuarantinedMedia, err = store.sqlDb.Prepare(selectQuarantinedMedia); err != nil {
		return nil, err
	}
	if store.stmts.selectServerQuarantinedMedia, err = store.sqlDb.Prepare(selectServerQuarantinedMedia); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaByUser, err = store.sqlDb.Prepare(selectMediaByUser); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaByUserBefore, err = store.sqlDb.Prepare(selectMediaByUserBefore); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaByDomainBefore, err = store.sqlDb.Prepare(selectMediaByDomainBefore); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaByLocation, err = store.sqlDb.Prepare(selectMediaByLocation); err != nil {
		return nil, err
	}
	if store.stmts.selectIfQuarantined, err = store.sqlDb.Prepare(selectIfQuarantined); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *MediaStoreFactory) Create(ctx rcontext.RequestContext) *MediaStore {
	return &MediaStore{
		factory:    f,
		ctx:        ctx,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *MediaStore) Insert(media *types.Media) error {
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

func (s *MediaStore) Delete(origin string, mediaId string) error {
	_, err := s.statements.deleteMedia.ExecContext(s.ctx, origin, mediaId)
	return err
}

func (s *MediaStore) SetQuarantined(origin string, mediaId string, isQuarantined bool) error {
	_, err := s.statements.updateQuarantined.ExecContext(s.ctx, origin, mediaId, isQuarantined)
	return err
}

func (s *MediaStore) UpdateDatastoreAndLocation(media *types.Media) error {
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
	if v, ok := dsCacheById.Load(id); ok {
		ds := v.(*types.Datastore)
		return &types.Datastore{
			DatastoreId: ds.DatastoreId,
			Type:        ds.Type,
			Uri:         ds.Uri,
		}, nil
	}

	d := &types.Datastore{}
	err := s.statements.selectDatastore.QueryRowContext(s.ctx, id).Scan(
		&d.DatastoreId,
		&d.Type,
		&d.Uri,
	)
	if err != nil {
		dsCacheById.Store(d.Uri, d)
		dsCacheByPath.Store(d.Uri, d)
	}

	return &types.Datastore{
		DatastoreId: d.DatastoreId,
		Type:        d.Type,
		Uri:         d.Uri,
	}, err
}

func (s *MediaStore) InsertDatastore(datastore *types.Datastore) error {
	_, err := s.statements.insertDatastore.ExecContext(
		s.ctx,
		datastore.DatastoreId,
		datastore.Type,
		datastore.Uri,
	)
	if err != nil {
		d := &types.Datastore{
			DatastoreId: datastore.DatastoreId,
			Type:        datastore.Type,
			Uri:         datastore.Uri,
		}
		dsCacheById.Store(d.Uri, d)
		dsCacheByPath.Store(d.Uri, d)
	}
	return err
}

func (s *MediaStore) GetDatastoreByUri(uri string) (*types.Datastore, error) {
	if v, ok := dsCacheByPath.Load(uri); ok {
		ds := v.(*types.Datastore)
		return &types.Datastore{
			DatastoreId: ds.DatastoreId,
			Type:        ds.Type,
			Uri:         ds.Uri,
		}, nil
	}

	d := &types.Datastore{}
	err := s.statements.selectDatastoreByUri.QueryRowContext(s.ctx, uri).Scan(
		&d.DatastoreId,
		&d.Type,
		&d.Uri,
	)
	if err != nil {
		dsCacheById.Store(d.Uri, d)
		dsCacheByPath.Store(d.Uri, d)
	}

	return &types.Datastore{
		DatastoreId: d.DatastoreId,
		Type:        d.Type,
		Uri:         d.Uri,
	}, err
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

func (s *MediaStore) GetAllDatastores() ([]*types.Datastore, error) {
	rows, err := s.statements.selectAllDatastores.QueryContext(s.ctx)
	if err != nil {
		return nil, err
	}

	var results []*types.Datastore
	for rows.Next() {
		obj := &types.Datastore{}
		err = rows.Scan(
			&obj.DatastoreId,
			&obj.Type,
			&obj.Uri,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *MediaStore) GetAllMediaForServer(serverName string) ([]*types.Media, error) {
	rows, err := s.statements.selectAllMediaForServer.QueryContext(s.ctx, serverName)
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

func (s *MediaStore) GetAllMediaForServerUsers(serverName string, userIds []string) ([]*types.Media, error) {
	rows, err := s.statements.selectAllMediaForServerUsers.QueryContext(s.ctx, serverName, pq.Array(userIds))
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

func (s *MediaStore) GetAllMediaInIds(serverName string, mediaIds []string) ([]*types.Media, error) {
	rows, err := s.statements.selectAllMediaForServerIds.QueryContext(s.ctx, serverName, pq.Array(mediaIds))
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

func (s *MediaStore) GetAllQuarantinedMedia() ([]*types.Media, error) {
	rows, err := s.statements.selectQuarantinedMedia.QueryContext(s.ctx)
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

func (s *MediaStore) GetQuarantinedMediaFor(serverName string) ([]*types.Media, error) {
	rows, err := s.statements.selectServerQuarantinedMedia.QueryContext(s.ctx, serverName)
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

func (s *MediaStore) GetMediaByUser(userId string) ([]*types.Media, error) {
	rows, err := s.statements.selectMediaByUser.QueryContext(s.ctx, userId)
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

func (s *MediaStore) GetMediaByUserBefore(userId string, beforeTs int64) ([]*types.Media, error) {
	rows, err := s.statements.selectMediaByUserBefore.QueryContext(s.ctx, userId, beforeTs)
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

func (s *MediaStore) GetMediaByDomainBefore(serverName string, beforeTs int64) ([]*types.Media, error) {
	rows, err := s.statements.selectMediaByDomainBefore.QueryContext(s.ctx, serverName, beforeTs)
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

func (s *MediaStore) GetMediaByLocation(datastoreId string, location string) ([]*types.Media, error) {
	rows, err := s.statements.selectMediaByLocation.QueryContext(s.ctx, datastoreId, location)
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

func (s *MediaStore) IsQuarantined(sha256hash string) (bool, error) {
	r := s.statements.selectIfQuarantined.QueryRow(sha256hash, true)
	var i int
	err := r.Scan(&i)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
