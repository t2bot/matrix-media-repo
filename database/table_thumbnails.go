package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbMethod string

type DbThumbnail struct {
	*Locatable
	Origin      string
	MediaId     string
	ContentType string
	Width       int
	Height      int
	Method      string
	Animated    bool
	//Sha256Hash  string
	SizeBytes  int64
	CreationTs int64
	//DatastoreId string
	//Location    string
}

const selectThumbnailByParams = "SELECT origin, media_id, content_type, width, height, method, animated, sha256_hash, size_bytes, creation_ts, datastore_id, location FROM thumbnails WHERE origin = $1 AND media_id = $2 AND width = $3 AND height = $4 AND method = $5 AND animated = $6;"
const insertThumbnail = "INSERT INTO thumbnails (origin, media_id, content_type, width, height, method, animated, sha256_hash, size_bytes, creation_ts, datastore_id, location) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);"

type thumbnailsTableStatements struct {
	selectThumbnailByParams *sql.Stmt
	insertThumbnail         *sql.Stmt
}

type thumbnailsTableWithContext struct {
	statements *thumbnailsTableStatements
	ctx        rcontext.RequestContext
}

func prepareThumbnailsTables(db *sql.DB) (*thumbnailsTableStatements, error) {
	var err error
	var stmts = &thumbnailsTableStatements{}

	if stmts.selectThumbnailByParams, err = db.Prepare(selectThumbnailByParams); err != nil {
		return nil, errors.New("error preparing selectThumbnailByParams: " + err.Error())
	}
	if stmts.insertThumbnail, err = db.Prepare(insertThumbnail); err != nil {
		return nil, errors.New("error preparing insertThumbnail: " + err.Error())
	}

	return stmts, nil
}

func (s *thumbnailsTableStatements) Prepare(ctx rcontext.RequestContext) *thumbnailsTableWithContext {
	return &thumbnailsTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *thumbnailsTableWithContext) GetByParams(origin string, mediaId string, width int, height int, method string, animated bool) (*DbThumbnail, error) {
	row := s.statements.selectThumbnailByParams.QueryRowContext(s.ctx, origin, mediaId, width, height, method, animated)
	val := &DbThumbnail{Locatable: &Locatable{}}
	err := row.Scan(&val.Origin, &val.MediaId, &val.ContentType, &val.Width, &val.Height, &val.Method, &val.Animated, &val.Sha256Hash, &val.SizeBytes, &val.CreationTs, &val.DatastoreId, &val.Location)
	if err == sql.ErrNoRows {
		err = nil
		val = nil
	}
	return val, err
}

func (s *thumbnailsTableWithContext) Insert(record *DbThumbnail) error {
	_, err := s.statements.insertThumbnail.ExecContext(s.ctx, record.Origin, record.MediaId, record.ContentType, record.Width, record.Height, record.Method, record.Animated, record.Sha256Hash, record.SizeBytes, record.CreationTs, record.DatastoreId, record.Location)
	return err
}
