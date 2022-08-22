package stores

import (
	"database/sql"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
)

const selectThumbnail = "SELECT origin, media_id, width, height, method, animated, content_type, size_bytes, datastore_id, location, creation_ts, sha256_hash FROM thumbnails WHERE origin = $1 and media_id = $2 and width = $3 and height = $4 and method = $5 and animated = $6;"
const insertThumbnail = "INSERT INTO thumbnails (origin, media_id, width, height, method, animated, content_type, size_bytes, datastore_id, location, creation_ts, sha256_hash) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);"
const updateThumbnailHash = "UPDATE thumbnails SET sha256_hash = $7 WHERE origin = $1 and media_id = $2 and width = $3 and height = $4 and method = $5 and animated = $6;"
const selectThumbnailsWithoutHash = "SELECT origin, media_id, width, height, method, animated, content_type, size_bytes, datastore_id, location, creation_ts, sha256_hash FROM thumbnails WHERE sha256_hash IS NULL OR sha256_hash = '';"
const selectThumbnailsWithoutDatastore = "SELECT origin, media_id, width, height, method, animated, content_type, size_bytes, datastore_id, location, creation_ts, sha256_hash FROM thumbnails WHERE datastore_id IS NULL OR datastore_id = '';"
const updateThumbnailDatastoreAndLocation = "UPDATE thumbnails SET location = $8, datastore_id = $7 WHERE origin = $1 and media_id = $2 and width = $3 and height = $4 and method = $5 and animated = $6;"
const selectThumbnailsForMedia = "SELECT origin, media_id, width, height, method, animated, content_type, size_bytes, datastore_id, location, creation_ts, sha256_hash FROM thumbnails WHERE origin = $1 AND media_id = $2;"
const deleteThumbnailsForMedia = "DELETE FROM thumbnails WHERE origin = $1 AND media_id = $2;"
const selectThumbnailsCreatedBefore = "SELECT origin, media_id, width, height, method, animated, content_type, size_bytes, datastore_id, location, creation_ts, sha256_hash FROM thumbnails WHERE creation_ts < $1;"
const deleteThumbnailsWithHash = "DELETE FROM thumbnails WHERE sha256_hash = $1;"
const selectThumbnailLocationsForDatastore = "SELECT distinct location FROM thumbnails WHERE datastore_id = $1;"

type thumbnailStatements struct {
	selectThumbnail                      *sql.Stmt
	insertThumbnail                      *sql.Stmt
	updateThumbnailHash                  *sql.Stmt
	selectThumbnailsWithoutHash          *sql.Stmt
	selectThumbnailsWithoutDatastore     *sql.Stmt
	updateThumbnailDatastoreAndLocation  *sql.Stmt
	selectThumbnailsForMedia             *sql.Stmt
	deleteThumbnailsForMedia             *sql.Stmt
	selectThumbnailsCreatedBefore        *sql.Stmt
	deleteThumbnailsWithHash             *sql.Stmt
	selectThumbnailLocationsForDatastore *sql.Stmt
}

type ThumbnailStoreFactory struct {
	sqlDb *sql.DB
	stmts *thumbnailStatements
}

type ThumbnailStore struct {
	factory    *ThumbnailStoreFactory // just for reference
	ctx        rcontext.RequestContext
	statements *thumbnailStatements // copied from factory
}

func InitThumbnailStore(sqlDb *sql.DB) (*ThumbnailStoreFactory, error) {
	store := ThumbnailStoreFactory{stmts: &thumbnailStatements{}}
	var err error

	store.sqlDb = sqlDb

	if store.stmts.selectThumbnail, err = store.sqlDb.Prepare(selectThumbnail); err != nil {
		return nil, err
	}
	if store.stmts.insertThumbnail, err = store.sqlDb.Prepare(insertThumbnail); err != nil {
		return nil, err
	}
	if store.stmts.updateThumbnailHash, err = store.sqlDb.Prepare(updateThumbnailHash); err != nil {
		return nil, err
	}
	if store.stmts.selectThumbnailsWithoutHash, err = store.sqlDb.Prepare(selectThumbnailsWithoutHash); err != nil {
		return nil, err
	}
	if store.stmts.selectThumbnailsWithoutDatastore, err = store.sqlDb.Prepare(selectThumbnailsWithoutDatastore); err != nil {
		return nil, err
	}
	if store.stmts.updateThumbnailDatastoreAndLocation, err = store.sqlDb.Prepare(updateThumbnailDatastoreAndLocation); err != nil {
		return nil, err
	}
	if store.stmts.selectThumbnailsForMedia, err = store.sqlDb.Prepare(selectThumbnailsForMedia); err != nil {
		return nil, err
	}
	if store.stmts.deleteThumbnailsForMedia, err = store.sqlDb.Prepare(deleteThumbnailsForMedia); err != nil {
		return nil, err
	}
	if store.stmts.selectThumbnailsCreatedBefore, err = store.sqlDb.Prepare(selectThumbnailsCreatedBefore); err != nil {
		return nil, err
	}
	if store.stmts.deleteThumbnailsWithHash, err = store.sqlDb.Prepare(deleteThumbnailsWithHash); err != nil {
		return nil, err
	}
	if store.stmts.selectThumbnailLocationsForDatastore, err = store.sqlDb.Prepare(selectThumbnailLocationsForDatastore); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *ThumbnailStoreFactory) New(ctx rcontext.RequestContext) *ThumbnailStore {
	return &ThumbnailStore{
		factory:    f,
		ctx:        ctx,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *ThumbnailStore) Insert(thumbnail *types.Thumbnail) error {
	_, err := s.statements.insertThumbnail.ExecContext(
		s.ctx,
		thumbnail.Origin,
		thumbnail.MediaId,
		thumbnail.Width,
		thumbnail.Height,
		thumbnail.Method,
		thumbnail.Animated,
		thumbnail.ContentType,
		thumbnail.SizeBytes,
		thumbnail.DatastoreId,
		thumbnail.Location,
		thumbnail.CreationTs,
		thumbnail.Sha256Hash,
	)

	return err
}

func (s *ThumbnailStore) Get(origin string, mediaId string, width int, height int, method string, animated bool) (*types.Thumbnail, error) {
	t := &types.Thumbnail{}
	err := s.statements.selectThumbnail.QueryRowContext(s.ctx, origin, mediaId, width, height, method, animated).Scan(
		&t.Origin,
		&t.MediaId,
		&t.Width,
		&t.Height,
		&t.Method,
		&t.Animated,
		&t.ContentType,
		&t.SizeBytes,
		&t.DatastoreId,
		&t.Location,
		&t.CreationTs,
		&t.Sha256Hash,
	)
	return t, err
}

func (s *ThumbnailStore) UpdateHash(thumbnail *types.Thumbnail) error {
	_, err := s.statements.updateThumbnailHash.ExecContext(
		s.ctx,
		thumbnail.Origin,
		thumbnail.MediaId,
		thumbnail.Width,
		thumbnail.Height,
		thumbnail.Method,
		thumbnail.Animated,
		thumbnail.Sha256Hash,
	)

	return err
}

func (s *ThumbnailStore) UpdateDatastoreAndLocation(thumbnail *types.Thumbnail) error {
	_, err := s.statements.updateThumbnailDatastoreAndLocation.ExecContext(
		s.ctx,
		thumbnail.Origin,
		thumbnail.MediaId,
		thumbnail.Width,
		thumbnail.Height,
		thumbnail.Method,
		thumbnail.Animated,
		thumbnail.DatastoreId,
		thumbnail.Location,
	)

	return err
}

func (s *ThumbnailStore) GetAllWithoutHash() ([]*types.Thumbnail, error) {
	rows, err := s.statements.selectThumbnailsWithoutHash.QueryContext(s.ctx)
	if err != nil {
		return nil, err
	}

	var results []*types.Thumbnail
	for rows.Next() {
		obj := &types.Thumbnail{}
		err = rows.Scan(
			&obj.Origin,
			&obj.MediaId,
			&obj.Width,
			&obj.Height,
			&obj.Method,
			&obj.Animated,
			&obj.ContentType,
			&obj.SizeBytes,
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.Sha256Hash,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *ThumbnailStore) GetAllWithoutDatastore() ([]*types.Thumbnail, error) {
	rows, err := s.statements.selectThumbnailsWithoutDatastore.QueryContext(s.ctx)
	if err != nil {
		return nil, err
	}

	var results []*types.Thumbnail
	for rows.Next() {
		obj := &types.Thumbnail{}
		err = rows.Scan(
			&obj.Origin,
			&obj.MediaId,
			&obj.Width,
			&obj.Height,
			&obj.Method,
			&obj.Animated,
			&obj.ContentType,
			&obj.SizeBytes,
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.Sha256Hash,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *ThumbnailStore) GetAllForMedia(origin string, mediaId string) ([]*types.Thumbnail, error) {
	rows, err := s.statements.selectThumbnailsForMedia.QueryContext(s.ctx, origin, mediaId)
	if err != nil {
		return nil, err
	}

	var results []*types.Thumbnail
	for rows.Next() {
		obj := &types.Thumbnail{}
		err = rows.Scan(
			&obj.Origin,
			&obj.MediaId,
			&obj.Width,
			&obj.Height,
			&obj.Method,
			&obj.Animated,
			&obj.ContentType,
			&obj.SizeBytes,
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.Sha256Hash,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *ThumbnailStore) DeleteAllForMedia(origin string, mediaId string) error {
	_, err := s.statements.deleteThumbnailsForMedia.ExecContext(s.ctx, origin, mediaId)
	if err != nil {
		return err
	}
	return nil
}

func (s *ThumbnailStore) GetOldThumbnails(beforeTs int64) ([]*types.Thumbnail, error) {
	rows, err := s.statements.selectThumbnailsCreatedBefore.QueryContext(s.ctx, beforeTs)
	if err != nil {
		return nil, err
	}

	var results []*types.Thumbnail
	for rows.Next() {
		obj := &types.Thumbnail{}
		err = rows.Scan(
			&obj.Origin,
			&obj.MediaId,
			&obj.Width,
			&obj.Height,
			&obj.Method,
			&obj.Animated,
			&obj.ContentType,
			&obj.SizeBytes,
			&obj.DatastoreId,
			&obj.Location,
			&obj.CreationTs,
			&obj.Sha256Hash,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}

	return results, nil
}

func (s *ThumbnailStore) DeleteWithHash(sha256hash string) error {
	_, err := s.statements.deleteThumbnailsWithHash.ExecContext(s.ctx, sha256hash)
	if err != nil {
		return err
	}
	return nil
}

func (s *ThumbnailStore) GetDistinctLocationsForDatastore(datastoreId string) ([]string, error) {
	rows, err := s.statements.selectThumbnailLocationsForDatastore.QueryContext(s.ctx, datastoreId)
	if err != nil {
		return nil, err
	}

	locations := make([]string, 0)
	for rows.Next() {
		s := ""
		err = rows.Scan(&s)
		if err != nil {
			return nil, err
		}
		locations = append(locations, s)
	}

	return locations, nil
}
