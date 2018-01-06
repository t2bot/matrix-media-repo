package stores

import (
	"context"
	"database/sql"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
)

const selectThumbnail = "SELECT origin, media_id, width, height, method, content_type, size_bytes, location, creation_ts FROM thumbnails WHERE origin = $1 and media_id = $2 and width = $3 and height = $4 and method = $5;"
const insertThumbnail = "INSERT INTO thumbnails (origin, media_id, width, height, method, content_type, size_bytes, location, creation_ts) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);"

type thumbnailStatements struct {
	selectThumbnail *sql.Stmt
	insertThumbnail *sql.Stmt
}

type ThumbnailStoreFactory struct {
	sqlDb *sql.DB
	stmts *thumbnailStatements
}

type ThumbnailStore struct {
	factory    *ThumbnailStoreFactory // just for reference
	ctx        context.Context
	log        *logrus.Entry
	statements *thumbnailStatements // copied from factory
}

func InitThumbnailStore(sqlDb *sql.DB) (*ThumbnailStoreFactory, error) {
	store := ThumbnailStoreFactory{stmts:&thumbnailStatements{}}
	var err error

	store.sqlDb = sqlDb

	if store.stmts.selectThumbnail, err = store.sqlDb.Prepare(selectThumbnail); err != nil {
		return nil, err
	}
	if store.stmts.insertThumbnail, err = store.sqlDb.Prepare(insertThumbnail); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *ThumbnailStoreFactory) Create(ctx context.Context, entry *logrus.Entry) (*ThumbnailStore) {
	return &ThumbnailStore{
		factory:    f,
		ctx:        ctx,
		log:        entry,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *ThumbnailStore) Insert(thumbnail *types.Thumbnail) (error) {
	_, err := s.statements.insertThumbnail.ExecContext(
		s.ctx,
		thumbnail.Origin,
		thumbnail.MediaId,
		thumbnail.Width,
		thumbnail.Height,
		thumbnail.Method,
		thumbnail.ContentType,
		thumbnail.SizeBytes,
		thumbnail.Location,
		thumbnail.CreationTs,
	)

	return err
}

func (s *ThumbnailStore) Get(origin string, mediaId string, width int, height int, method string) (types.Thumbnail, error) {
	t := &types.Thumbnail{}
	err := s.statements.selectThumbnail.QueryRowContext(s.ctx, origin, mediaId, width, height, method).Scan(
		&t.Origin,
		&t.MediaId,
		&t.Width,
		&t.Height,
		&t.Method,
		&t.ContentType,
		&t.SizeBytes,
		&t.Location,
		&t.CreationTs,
	)
	return *t, err
}
