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
const selectThumbnailByLocationExists = "SELECT TRUE FROM thumbnails WHERE datastore_id = $1 AND location = $2 LIMIT 1;"
const selectThumbnailsForMedia = "SELECT origin, media_id, content_type, width, height, method, animated, sha256_hash, size_bytes, creation_ts, datastore_id, location FROM thumbnails WHERE origin = $1 AND media_id = $2;"
const selectOldThumbnails = "SELECT origin, media_id, content_type, width, height, method, animated, sha256_hash, size_bytes, creation_ts, datastore_id, location FROM thumbnails WHERE sha256_hash IN (SELECT t2.sha256_hash FROM thumbnails AS t2 WHERE t2.creation_ts < $1);"
const deleteThumbnail = "DELETE FROM thumbnails WHERE origin = $1 AND media_id = $2 AND content_type = $3 AND width = $4 AND height = $5 AND method = $6 AND animated = $7 AND sha256_hash = $8 AND size_bytes = $9 AND creation_ts = $10 AND datastore_id = $11 AND location = $11;"
const updateThumbnailLocation = "UPDATE thumbnails SET datastore_id = $3, location = $4 WHERE datastore_id = $1 AND location = $2;"

type thumbnailsTableStatements struct {
	selectThumbnailByParams         *sql.Stmt
	insertThumbnail                 *sql.Stmt
	selectThumbnailByLocationExists *sql.Stmt
	selectThumbnailsForMedia        *sql.Stmt
	selectOldThumbnails             *sql.Stmt
	deleteThumbnail                 *sql.Stmt
	updateThumbnailLocation         *sql.Stmt
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
	if stmts.selectThumbnailByLocationExists, err = db.Prepare(selectThumbnailByLocationExists); err != nil {
		return nil, errors.New("error preparing selectThumbnailByLocationExists: " + err.Error())
	}
	if stmts.selectThumbnailsForMedia, err = db.Prepare(selectThumbnailsForMedia); err != nil {
		return nil, errors.New("error preparing selectThumbnailsForMedia: " + err.Error())
	}
	if stmts.selectOldThumbnails, err = db.Prepare(selectOldThumbnails); err != nil {
		return nil, errors.New("error preparing selectOldThumbnails: " + err.Error())
	}
	if stmts.deleteThumbnail, err = db.Prepare(deleteThumbnail); err != nil {
		return nil, errors.New("error preparing deleteThumbnail: " + err.Error())
	}
	if stmts.updateThumbnailLocation, err = db.Prepare(updateThumbnailLocation); err != nil {
		return nil, errors.New("error preparing updateThumbnailLocation: " + err.Error())
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

func (s *thumbnailsTableWithContext) GetForMedia(origin string, mediaId string) ([]*DbThumbnail, error) {
	results := make([]*DbThumbnail, 0)
	rows, err := s.statements.selectThumbnailsForMedia.QueryContext(s.ctx, origin, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			return results, nil
		}
		return nil, err
	}
	for rows.Next() {
		val := &DbThumbnail{Locatable: &Locatable{}}
		if err = rows.Scan(&val.Origin, &val.MediaId, &val.ContentType, &val.Width, &val.Height, &val.Method, &val.Animated, &val.Sha256Hash, &val.SizeBytes, &val.CreationTs, &val.DatastoreId, &val.Location); err != nil {
			return nil, err
		}
		results = append(results, val)
	}
	return results, nil
}

func (s *thumbnailsTableWithContext) GetOlderThan(ts int64) ([]*DbThumbnail, error) {
	results := make([]*DbThumbnail, 0)
	rows, err := s.statements.selectOldThumbnails.QueryContext(s.ctx, ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return results, nil
		}
		return nil, err
	}
	for rows.Next() {
		val := &DbThumbnail{Locatable: &Locatable{}}
		if err = rows.Scan(&val.Origin, &val.MediaId, &val.ContentType, &val.Width, &val.Height, &val.Method, &val.Animated, &val.Sha256Hash, &val.SizeBytes, &val.CreationTs, &val.DatastoreId, &val.Location); err != nil {
			return nil, err
		}
		results = append(results, val)
	}
	return results, nil
}

func (s *thumbnailsTableWithContext) Insert(record *DbThumbnail) error {
	_, err := s.statements.insertThumbnail.ExecContext(s.ctx, record.Origin, record.MediaId, record.ContentType, record.Width, record.Height, record.Method, record.Animated, record.Sha256Hash, record.SizeBytes, record.CreationTs, record.DatastoreId, record.Location)
	return err
}

func (s *thumbnailsTableWithContext) LocationExists(datastoreId string, location string) (bool, error) {
	row := s.statements.selectThumbnailByLocationExists.QueryRowContext(s.ctx, datastoreId, location)
	val := false
	err := row.Scan(&val)
	if err == sql.ErrNoRows {
		err = nil
		val = false
	}
	return val, err
}

func (s *thumbnailsTableWithContext) Delete(record *DbThumbnail) error {
	_, err := s.statements.deleteThumbnail.ExecContext(s.ctx, record.Origin, record.MediaId, record.ContentType, record.Width, record.Height, record.Method, record.Animated, record.Sha256Hash, record.SizeBytes, record.CreationTs, record.DatastoreId, record.Location)
	return err
}

func (s *thumbnailsTableWithContext) UpdateLocation(sourceDsId string, sourceLocation string, targetDsId string, targetLocation string) error {
	_, err := s.statements.updateThumbnailLocation.ExecContext(s.ctx, sourceDsId, sourceLocation, targetDsId, targetLocation)
	return err
}
