package stores

import (
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
)

type folderSize struct {
	Size int64
}

const selectSizeOfDatastore = "SELECT COALESCE(SUM(size_bytes), 0) + COALESCE((SELECT SUM(size_bytes) FROM thumbnails WHERE datastore_id = $1), 0) AS size_total FROM media WHERE datastore_id = $1;"
const upsertLastAccessed = "INSERT INTO last_access (sha256_hash, last_access_ts) VALUES ($1, $2) ON CONFLICT (sha256_hash) DO UPDATE SET last_access_ts = $2"

type metadataStoreStatements struct {
	upsertLastAccessed    *sql.Stmt
	selectSizeOfDatastore *sql.Stmt
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
