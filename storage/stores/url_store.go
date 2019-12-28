package stores

import (
	"database/sql"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

const selectUrlPreview = "SELECT url, error_code, bucket_ts, site_url, site_name, resource_type, description, title, image_mxc, image_type, image_size, image_width, image_height FROM url_previews WHERE url = $1 AND bucket_ts = $2;"
const insertUrlPreview = "INSERT INTO url_previews (url, error_code, bucket_ts, site_url, site_name, resource_type, description, title, image_mxc, image_type, image_size, image_width, image_height) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);"

type urlStatements struct {
	selectUrlPreview *sql.Stmt
	insertUrlPreview *sql.Stmt
}

type UrlStoreFactory struct {
	sqlDb *sql.DB
	stmts *urlStatements
}

type UrlStore struct {
	factory    *UrlStoreFactory // just for reference
	ctx        rcontext.RequestContext
	statements *urlStatements // copied from factory
}

func InitUrlStore(sqlDb *sql.DB) (*UrlStoreFactory, error) {
	store := UrlStoreFactory{stmts: &urlStatements{}}
	var err error

	store.sqlDb = sqlDb

	if store.stmts.selectUrlPreview, err = store.sqlDb.Prepare(selectUrlPreview); err != nil {
		return nil, err
	}
	if store.stmts.insertUrlPreview, err = store.sqlDb.Prepare(insertUrlPreview); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *UrlStoreFactory) Create(ctx rcontext.RequestContext) *UrlStore {
	return &UrlStore{
		factory:    f,
		ctx:        ctx,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *UrlStore) GetPreview(url string, ts int64) (*types.CachedUrlPreview, error) {
	r := &types.CachedUrlPreview{
		Preview: &types.UrlPreview{},
	}
	err := s.statements.selectUrlPreview.QueryRowContext(s.ctx, url, GetBucketTs(ts)).Scan(
		&r.SearchUrl,
		&r.ErrorCode,
		&r.FetchedTs,
		&r.Preview.Url,
		&r.Preview.SiteName,
		&r.Preview.Type,
		&r.Preview.Description,
		&r.Preview.Title,
		&r.Preview.ImageMxc,
		&r.Preview.ImageType,
		&r.Preview.ImageSize,
		&r.Preview.ImageWidth,
		&r.Preview.ImageHeight,
	)

	return r, err
}

func (s *UrlStore) InsertPreview(record *types.CachedUrlPreview) error {
	_, err := s.statements.insertUrlPreview.ExecContext(
		s.ctx,
		record.SearchUrl,
		record.ErrorCode,
		GetBucketTs(record.FetchedTs),
		record.Preview.Url,
		record.Preview.SiteName,
		record.Preview.Type,
		record.Preview.Description,
		record.Preview.Title,
		record.Preview.ImageMxc,
		record.Preview.ImageType,
		record.Preview.ImageSize,
		record.Preview.ImageWidth,
		record.Preview.ImageHeight,
	)

	return err
}

func (s *UrlStore) InsertPreviewError(url string, errorCode string) error {
	return s.InsertPreview(&types.CachedUrlPreview{
		Preview:   &types.UrlPreview{},
		SearchUrl: url,
		ErrorCode: errorCode,
		FetchedTs: util.NowMillis(),
	})
}

func GetBucketTs(ts int64) int64 {
	// 1 hour buckets
	return (ts / 3600000) * 3600000
}
