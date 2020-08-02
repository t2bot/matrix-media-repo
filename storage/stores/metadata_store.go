package stores

import (
	"database/sql"
	"encoding/json"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type folderSize struct {
	Size int64
}

const selectSizeOfDatastore = "SELECT COALESCE(SUM(size_bytes), 0) + COALESCE((SELECT SUM(size_bytes) FROM thumbnails WHERE datastore_id = $1), 0) AS size_total FROM media WHERE datastore_id = $1;"
const upsertLastAccessed = "INSERT INTO last_access (sha256_hash, last_access_ts) VALUES ($1, $2) ON CONFLICT (sha256_hash) DO UPDATE SET last_access_ts = $2"
const selectMediaLastAccessedBeforeInDatastore = "SELECT m.sha256_hash, m.size_bytes, m.datastore_id, m.location, m.creation_ts, a.last_access_ts FROM media AS m JOIN last_access AS a ON m.sha256_hash = a.sha256_hash WHERE a.last_access_ts < $1 AND m.datastore_id = $2"
const selectThumbnailsLastAccessedBeforeInDatastore = "SELECT m.sha256_hash, m.size_bytes, m.datastore_id, m.location, m.creation_ts, a.last_access_ts FROM thumbnails AS m JOIN last_access AS a ON m.sha256_hash = a.sha256_hash WHERE a.last_access_ts < $1 AND m.datastore_id = $2"
const changeDatastoreOfMediaHash = "UPDATE media SET datastore_id = $1, location = $2 WHERE sha256_hash = $3"
const changeDatastoreOfThumbnailHash = "UPDATE thumbnails SET datastore_id = $1, location = $2 WHERE sha256_hash = $3"
const selectUploadCountsForServer = "SELECT COALESCE((SELECT COUNT(origin) FROM media WHERE origin = $1), 0) AS media, COALESCE((SELECT COUNT(origin) FROM thumbnails WHERE origin = $1), 0) AS thumbnails"
const selectUploadSizesForServer = "SELECT COALESCE((SELECT SUM(size_bytes) FROM media WHERE origin = $1), 0) AS media, COALESCE((SELECT SUM(size_bytes) FROM thumbnails WHERE origin = $1), 0) AS thumbnails"
const selectUsersForServer = "SELECT DISTINCT user_id FROM media WHERE origin = $1 AND user_id IS NOT NULL AND LENGTH(user_id) > 0"
const insertNewBackgroundTask = "INSERT INTO background_tasks (task, params, start_ts) VALUES ($1, $2, $3) RETURNING id;"
const selectBackgroundTask = "SELECT id, task, params, start_ts, end_ts FROM background_tasks WHERE id = $1"
const updateBackgroundTask = "UPDATE background_tasks SET end_ts = $2 WHERE id = $1"
const selectAllBackgroundTasks = "SELECT id, task, params, start_ts, end_ts FROM background_tasks"
const insertReservation = "INSERT INTO reserved_media (origin, media_id, reason) VALUES ($1, $2, $3);"
const selectReservation = "SELECT origin, media_id, reason FROM reserved_media WHERE origin = $1 AND media_id = $2;"
const selectMediaLastAccessed = "SELECT m.sha256_hash, m.size_bytes, m.datastore_id, m.location, m.creation_ts, a.last_access_ts FROM media AS m JOIN last_access AS a ON m.sha256_hash = a.sha256_hash WHERE a.last_access_ts < $1;"
const insertBlurhash = "INSERT INTO blurhashes (sha256_hash, blurhash) VALUES ($1, $2);"
const selectBlurhash = "SELECT blurhash FROM blurhashes WHERE sha256_hash = $1;"
const selectUserStats = "SELECT user_id, uploaded_bytes FROM user_stats WHERE user_id = $1;"

type metadataStoreStatements struct {
	upsertLastAccessed                            *sql.Stmt
	selectSizeOfDatastore                         *sql.Stmt
	selectMediaLastAccessedBeforeInDatastore      *sql.Stmt
	selectThumbnailsLastAccessedBeforeInDatastore *sql.Stmt
	changeDatastoreOfMediaHash                    *sql.Stmt
	changeDatastoreOfThumbnailHash                *sql.Stmt
	selectUploadCountsForServer                   *sql.Stmt
	selectUploadSizesForServer                    *sql.Stmt
	selectUsersForServer                          *sql.Stmt
	insertNewBackgroundTask                       *sql.Stmt
	selectBackgroundTask                          *sql.Stmt
	updateBackgroundTask                          *sql.Stmt
	selectAllBackgroundTasks                      *sql.Stmt
	insertReservation                             *sql.Stmt
	selectReservation                             *sql.Stmt
	selectMediaLastAccessed                       *sql.Stmt
	insertBlurhash                                *sql.Stmt
	selectBlurhash                                *sql.Stmt
	selectUserStats                               *sql.Stmt
}

type MetadataStoreFactory struct {
	sqlDb *sql.DB
	stmts *metadataStoreStatements
}

type MetadataStore struct {
	factory    *MetadataStoreFactory // just for reference
	ctx        rcontext.RequestContext
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
	if store.stmts.changeDatastoreOfMediaHash, err = store.sqlDb.Prepare(changeDatastoreOfMediaHash); err != nil {
		return nil, err
	}
	if store.stmts.changeDatastoreOfThumbnailHash, err = store.sqlDb.Prepare(changeDatastoreOfThumbnailHash); err != nil {
		return nil, err
	}
	if store.stmts.selectUsersForServer, err = store.sqlDb.Prepare(selectUsersForServer); err != nil {
		return nil, err
	}
	if store.stmts.selectUploadSizesForServer, err = store.sqlDb.Prepare(selectUploadSizesForServer); err != nil {
		return nil, err
	}
	if store.stmts.selectUploadCountsForServer, err = store.sqlDb.Prepare(selectUploadCountsForServer); err != nil {
		return nil, err
	}
	if store.stmts.insertNewBackgroundTask, err = store.sqlDb.Prepare(insertNewBackgroundTask); err != nil {
		return nil, err
	}
	if store.stmts.selectBackgroundTask, err = store.sqlDb.Prepare(selectBackgroundTask); err != nil {
		return nil, err
	}
	if store.stmts.updateBackgroundTask, err = store.sqlDb.Prepare(updateBackgroundTask); err != nil {
		return nil, err
	}
	if store.stmts.selectAllBackgroundTasks, err = store.sqlDb.Prepare(selectAllBackgroundTasks); err != nil {
		return nil, err
	}
	if store.stmts.insertReservation, err = store.sqlDb.Prepare(insertReservation); err != nil {
		return nil, err
	}
	if store.stmts.selectReservation, err = store.sqlDb.Prepare(selectReservation); err != nil {
		return nil, err
	}
	if store.stmts.selectMediaLastAccessed, err = store.sqlDb.Prepare(selectMediaLastAccessed); err != nil {
		return nil, err
	}
	if store.stmts.insertBlurhash, err = store.sqlDb.Prepare(insertBlurhash); err != nil {
		return nil, err
	}
	if store.stmts.selectBlurhash, err = store.sqlDb.Prepare(selectBlurhash); err != nil {
		return nil, err
	}
	if store.stmts.selectUserStats, err = store.sqlDb.Prepare(selectUserStats); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *MetadataStoreFactory) Create(ctx rcontext.RequestContext) *MetadataStore {
	return &MetadataStore{
		factory:    f,
		ctx:        ctx,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *MetadataStore) UpsertLastAccess(sha256Hash string, timestamp int64) error {
	_, err := s.statements.upsertLastAccessed.ExecContext(s.ctx, sha256Hash, timestamp)
	return err
}

func (s *MetadataStore) ChangeDatastoreOfHash(datastoreId string, location string, sha256hash string) error {
	_, err1 := s.statements.changeDatastoreOfMediaHash.ExecContext(s.ctx, datastoreId, location, sha256hash)
	if err1 != nil {
		return err1
	}
	_, err2 := s.statements.changeDatastoreOfThumbnailHash.ExecContext(s.ctx, datastoreId, location, sha256hash)
	if err2 != nil {
		return err2
	}
	return nil
}

func (s *MetadataStore) GetEstimatedSizeOfDatastore(datastoreId string) (int64, error) {
	r := &folderSize{}
	err := s.statements.selectSizeOfDatastore.QueryRowContext(s.ctx, datastoreId).Scan(&r.Size)
	return r.Size, err
}

func (s *MetadataStore) GetOldMedia(beforeTs int64) ([]*types.MinimalMediaMetadata, error) {
	rows, err := s.statements.selectMediaLastAccessed.QueryContext(s.ctx, beforeTs)
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

func (s *MetadataStore) GetUsersForServer(serverName string) ([]string, error) {
	rows, err := s.statements.selectUsersForServer.QueryContext(s.ctx, serverName)
	if err != nil {
		return nil, err
	}

	results := make([]string, 0)
	for rows.Next() {
		v := ""
		err = rows.Scan(&v)
		if err != nil {
			return nil, err
		}
		results = append(results, v)
	}

	return results, nil
}

func (s *MetadataStore) GetByteUsageForServer(serverName string) (int64, int64, error) {
	row := s.statements.selectUploadSizesForServer.QueryRowContext(s.ctx, serverName)

	media := int64(0)
	thumbs := int64(0)
	err := row.Scan(&media, &thumbs)
	if err != nil {
		return 0, 0, err
	}

	return media, thumbs, nil
}

func (s *MetadataStore) GetCountUsageForServer(serverName string) (int64, int64, error) {
	row := s.statements.selectUploadCountsForServer.QueryRowContext(s.ctx, serverName)

	media := int64(0)
	thumbs := int64(0)
	err := row.Scan(&media, &thumbs)
	if err != nil {
		return 0, 0, err
	}

	return media, thumbs, nil
}

func (s *MetadataStore) CreateBackgroundTask(name string, params map[string]interface{}) (*types.BackgroundTask, error) {
	now := util.NowMillis()
	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	r := s.statements.insertNewBackgroundTask.QueryRowContext(s.ctx, name, string(b), now)
	var id int
	err = r.Scan(&id)
	if err != nil {
		return nil, err
	}
	return &types.BackgroundTask{
		ID:      id,
		Name:    name,
		StartTs: now,
		EndTs:   0,
	}, nil
}

func (s *MetadataStore) FinishedBackgroundTask(id int) error {
	now := util.NowMillis()
	_, err := s.statements.updateBackgroundTask.ExecContext(s.ctx, id, now)
	return err
}

func (s *MetadataStore) GetBackgroundTask(id int) (*types.BackgroundTask, error) {
	r := s.statements.selectBackgroundTask.QueryRowContext(s.ctx, id)
	task := &types.BackgroundTask{}
	var paramsStr string
	var endTs sql.NullInt64

	err := r.Scan(&task.ID, &task.Name, &paramsStr, &task.StartTs, &endTs)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(paramsStr), &task.Params)
	if err != nil {
		return nil, err
	}

	if endTs.Valid {
		task.EndTs = endTs.Int64
	}

	return task, nil
}

func (s *MetadataStore) GetAllBackgroundTasks() ([]*types.BackgroundTask, error) {
	rows, err := s.statements.selectAllBackgroundTasks.QueryContext(s.ctx)
	if err != nil {
		return nil, err
	}

	results := make([]*types.BackgroundTask, 0)
	for rows.Next() {
		task := &types.BackgroundTask{}
		var paramsStr string
		var endTs sql.NullInt64

		err := rows.Scan(&task.ID, &task.Name, &paramsStr, &task.StartTs, &endTs)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(paramsStr), &task.Params)
		if err != nil {
			return nil, err
		}

		if endTs.Valid {
			task.EndTs = endTs.Int64
		}

		results = append(results, task)
	}

	return results, nil
}

func (s *MetadataStore) ReserveMediaId(origin string, mediaId string, reason string) error {
	_, err := s.statements.insertReservation.ExecContext(s.ctx, origin, mediaId, reason)
	if err != nil {
		return err
	}
	return nil
}

func (s *MetadataStore) IsReserved(origin string, mediaId string) (bool, error) {
	r := s.statements.selectReservation.QueryRowContext(s.ctx, origin, mediaId)
	var dbOrigin string
	var dbMediaId string
	var dbReason string

	err := r.Scan(&dbOrigin, &dbMediaId, &dbReason)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	return true, nil
}

func (s *MetadataStore) InsertBlurhash(sha256Hash string, blurhash string) error {
	_, err := s.statements.insertBlurhash.ExecContext(s.ctx, sha256Hash, blurhash)
	if err != nil {
		return err
	}
	return nil
}

func (s *MetadataStore) GetBlurhash(sha256Hash string) (string, error) {
	r := s.statements.selectBlurhash.QueryRowContext(s.ctx, sha256Hash)
	var blurhash string

	err := r.Scan(&blurhash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return blurhash, nil
}

func (s *MetadataStore) GetUserStats(userId string) (*types.UserStats, error) {
	r := s.statements.selectUserStats.QueryRowContext(s.ctx, userId)

	stat := &types.UserStats{}
	err := r.Scan(
		&stat.UserId,
		&stat.UploadedBytes,
	)
	if err != nil {
		return nil, err
	}
	return stat, nil
}
