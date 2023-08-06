package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

type DbUrlPreview struct {
	Url            string
	ErrorCode      string
	BucketTs       int64
	SiteUrl        string
	SiteName       string
	ResourceType   string
	Description    string
	Title          string
	ImageMxc       string
	ImageType      string
	ImageSize      int64
	ImageWidth     int
	ImageHeight    int
	LanguageHeader string
}

const selectUrlPreview = "SELECT url, error_code, bucket_ts, site_url, site_name, resource_type, description, title, image_mxc, image_type, image_size, image_width, image_height, language_header FROM url_previews WHERE url = $1 AND bucket_ts = $2 AND language_header = $3;"
const insertUrlPreview = "INSERT INTO url_previews (url, error_code, bucket_ts, site_url, site_name, resource_type, description, title, image_mxc, image_type, image_size, image_width, image_height, language_header) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);"
const deleteOldUrlPreviews = "DELETE FROM url_previews WHERE bucket_ts <= $1;"

type urlPreviewsTableStatements struct {
	selectUrlPreview     *sql.Stmt
	insertUrlPreview     *sql.Stmt
	deleteOldUrlPreviews *sql.Stmt
}

type urlPreviewsTableWithContext struct {
	statements *urlPreviewsTableStatements
	ctx        rcontext.RequestContext
}

func prepareUrlPreviewsTables(db *sql.DB) (*urlPreviewsTableStatements, error) {
	var err error
	var stmts = &urlPreviewsTableStatements{}

	if stmts.selectUrlPreview, err = db.Prepare(selectUrlPreview); err != nil {
		return nil, errors.New("error preparing selectUrlPreview: " + err.Error())
	}
	if stmts.insertUrlPreview, err = db.Prepare(insertUrlPreview); err != nil {
		return nil, errors.New("error preparing insertUrlPreview: " + err.Error())
	}
	if stmts.deleteOldUrlPreviews, err = db.Prepare(deleteOldUrlPreviews); err != nil {
		return nil, errors.New("error preparing deleteOldUrlPreviews: " + err.Error())
	}

	return stmts, nil
}

func (s *urlPreviewsTableStatements) Prepare(ctx rcontext.RequestContext) *urlPreviewsTableWithContext {
	return &urlPreviewsTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *urlPreviewsTableWithContext) Get(url string, ts int64, languageHeader string) (*DbUrlPreview, error) {
	row := s.statements.selectUrlPreview.QueryRowContext(s.ctx, url, ts, languageHeader)
	val := &DbUrlPreview{}
	err := row.Scan(&val.Url, &val.ErrorCode, &val.BucketTs, &val.SiteUrl, &val.SiteName, &val.ResourceType, &val.Description, &val.Title, &val.ImageMxc, &val.ImageType, &val.ImageSize, &val.ImageWidth, &val.ImageHeight, &val.LanguageHeader)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return val, err
}

func (s *urlPreviewsTableWithContext) Insert(p *DbUrlPreview) error {
	_, err := s.statements.insertUrlPreview.ExecContext(s.ctx, p.Url, p.ErrorCode, p.BucketTs, p.SiteUrl, p.SiteName, p.ResourceType, p.Description, p.Title, p.ImageMxc, p.ImageType, p.ImageSize, p.ImageWidth, p.ImageHeight, p.LanguageHeader)
	return err
}

func (s *urlPreviewsTableWithContext) InsertError(url string, errorCode string) {
	_ = s.Insert(&DbUrlPreview{
		Url:       url,
		ErrorCode: errorCode,
		BucketTs:  util.GetHourBucket(util.NowMillis()),
		// remainder of fields don't matter
	})
}

func (s *urlPreviewsTableWithContext) DeleteOlderThan(ts int64) error {
	_, err := s.statements.deleteOldUrlPreviews.ExecContext(s.ctx, ts)
	return err
}
