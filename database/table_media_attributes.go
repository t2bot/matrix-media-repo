package database

import (
	"database/sql"
	"errors"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type DbMediaAttributes struct {
	Origin  string
	MediaId string
	Purpose Purpose
}

type Purpose string

const (
	PurposeNone   Purpose = "none"
	PurposePinned Purpose = "pinned"
)

func IsPurpose(purpose Purpose) bool {
	return purpose == PurposeNone || purpose == PurposePinned
}

const selectMediaAttributes = "SELECT origin, media_id, purpose FROM media_attributes WHERE origin = $1 AND media_id = $2;"
const upsertMediaPurpose = "INSERT INTO media_attributes (origin, media_id, purpose) VALUES ($1, $2, $3) ON CONFLICT (origin, media_id) DO UPDATE SET purpose = $3;"

type mediaAttributesTableStatements struct {
	selectMediaAttributes *sql.Stmt
	upsertMediaPurpose    *sql.Stmt
}

type mediaAttributesTableWithContext struct {
	statements *mediaAttributesTableStatements
	ctx        rcontext.RequestContext
}

func prepareMediaAttributesTables(db *sql.DB) (*mediaAttributesTableStatements, error) {
	var err error
	var stmts = &mediaAttributesTableStatements{}

	if stmts.selectMediaAttributes, err = db.Prepare(selectMediaAttributes); err != nil {
		return nil, errors.New("error preparing selectMediaAttributes: " + err.Error())
	}
	if stmts.upsertMediaPurpose, err = db.Prepare(upsertMediaPurpose); err != nil {
		return nil, errors.New("error preparing upsertMediaPurpose: " + err.Error())
	}

	return stmts, nil
}

func (s *mediaAttributesTableStatements) Prepare(ctx rcontext.RequestContext) *mediaAttributesTableWithContext {
	return &mediaAttributesTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *mediaAttributesTableWithContext) Get(origin string, mediaId string) (*DbMediaAttributes, error) {
	row := s.statements.selectMediaAttributes.QueryRowContext(s.ctx, origin, mediaId)
	val := &DbMediaAttributes{}
	err := row.Scan(&val.Origin, &val.MediaId, &val.Purpose)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = nil
	}
	return val, err
}

func (s *mediaAttributesTableWithContext) UpsertPurpose(origin string, mediaId string, purpose Purpose) error {
	_, err := s.statements.upsertMediaPurpose.ExecContext(s.ctx, origin, mediaId, purpose)
	return err
}
