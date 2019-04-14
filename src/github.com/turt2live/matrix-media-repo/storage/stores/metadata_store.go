package stores

import (
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
)

type folderSize struct {
	Size int64
}

const selectSizeOfDatastore = "SELECT COALESCE(SUM(size_bytes), 0) + COALESCE((SELECT SUM(size_bytes) FROM thumbnails WHERE datastore_id = $1), 0) AS size_total FROM media WHERE datastore_id = $1;"
const upsertLastAccessed = "INSERT INTO last_access (sha256_hash, last_access_ts) VALUES ($1, $2) ON CONFLICT (sha256_hash) DO UPDATE SET last_access_ts = $2"
const selectMediaLastAccessedBeforeInDatastore = "SELECT m.sha256_hash, m.size_bytes, m.location, m.datastore_id, m.creation_ts, a.last_access_ts FROM media AS m JOIN last_access AS a ON m.sha256_hash = a.sha256_hash WHERE a.last_access_ts < $1 AND m.datastore_id = $2";

const selectThumbnailsLastAccessedBeforeInDatastore = "SELECT m.sha256_hash, m.size_bytes, m.location, m.datastore_id, m.creation_ts, a.last_access_ts FROM thumbnails AS m JOIN last_access AS a ON m.sha256_hash = a.sha256_hash WHERE a.last_access_ts < $1 AND m.datastore_id = $2";

type metadataStoreStatements struct {
	upsertLastAccessed                            *sql.Stmt
	selectSizeOfDatastore                         *sql.Stmt
	selectMediaLastAccessedBeforeInDatastore      *sql.Stmt
	selectThumbnailsLastAccessedBeforeInDatastore *sql.Stmt
}

type MetadataStoreFactory struct {
	sqlDb *sql.DB
	stmts *metadataStoreStatements
}

type MetadataStore struct {
	factory    *MetadataStoreFactory // just for reference
	ctx        context.Context
	log        *logrus.Entry
	statements *metadataStoreStatements // copied from factory
}

func InitMetadataStore(sqlDb *sql.DB) (*MetadataStoreFactory, error) {
	store := MetadataStoreFactory{stmts: &metadataStoreStatements{}}
	var err error

	store.sqlDb = sqlDb

	if store.stmts.upsertLastAccessed, err = store.sqlDb.Prepare(upsertLastAccessed); err != nil {
		return nil, err
	}
	if store.stmts.selectSizeOfDatastore, err = store.sqlDb.Prepare(selectSizeOfDatastore); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaLastAccessedBeforeInDatastore, err = store.sqlDb.Prepare(selectMediaLastAccessedBeforeInDatastore); err != nil {
		return nil, err
	}
	if store.stmts.selectThumbnailsLastAccessedBeforeInDatastore, err = store.sqlDb.Prepare(selectThumbnailsLastAccessedBeforeInDatastore); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *MetadataStoreFactory) Create(ctx context.Context, entry *logrus.Entry) *MetadataStore {
	return &MetadataStore{
		factory:    f,
		ctx:        ctx,
		log:        entry,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *MetadataStore) UpsertLastAccess(sha256Hash string, timestamp int64) error {
	_, err := s.statements.upsertLastAccessed.ExecContext(s.ctx, sha256Hash, timestamp)
	return err
}

func (s *MetadataStore) GetEstimatedSizeOfDatastore(datastoreId string) (int64, error) {
	r := &folderSize{}
	err := s.statements.selectSizeOfDatastore.QueryRowContext(s.ctx, datastoreId).Scan(&r.Size)
	return r.Size, err
}

func (s *MetadataStore) GetOldMediaInDatastore(datastoreId string, beforeTs int64) ([]*types.MinimalMediaMetadata, error) {
	rows, err := s.statements.selectMediaLastAccessedBeforeInDatastore.QueryContext(s.ctx, beforeTs, datastoreId)
	if err != nil {
		return nil, err
	}

	var results []*types.MinimalMediaMetadata
	for rows.Next() {
		obj := &types.MinimalMediaMetadata{}
		err = rows.Scan(
			&obj.Sha256Hash,
			&obj.SizeBytes,
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.LastAccessTs,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *MetadataStore) GetOldThumbnailsInDatastore(datastoreId string, beforeTs int64) ([]*types.MinimalMediaMetadata, error) {
	rows, err := s.statements.selectThumbnailsLastAccessedBeforeInDatastore.QueryContext(s.ctx, beforeTs, datastoreId)
	if err != nil {
		return nil, err
	}

	var results []*types.MinimalMediaMetadata
	for rows.Next() {
		obj := &types.MinimalMediaMetadata{}
		err = rows.Scan(
			&obj.Sha256Hash,
			&obj.SizeBytes,
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.LastAccessTs,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}
