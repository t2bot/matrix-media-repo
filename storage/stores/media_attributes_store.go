package stores

import (
	"database/sql"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
)

const selectMediaAttributes = "SELECT origin, media_id, purpose FROM media_attributes WHERE origin = $1 AND media_id = $2;"
const upsertMediaPurpose = "INSERT INTO media_attributes (origin, media_id, purpose) VALUES ($1, $2, $3) ON CONFLICT (origin, media_id) DO UPDATE SET purpose = $3;"

type mediaAttributesStoreStatements struct {
	selectMediaAttributes *sql.Stmt
	upsertMediaPurpose    *sql.Stmt
}

type MediaAttributesStoreFactory struct {
	sqlDb *sql.DB
	stmts *mediaAttributesStoreStatements
}

type MediaAttributesStore struct {
	factory    *MediaAttributesStoreFactory // just for reference
	ctx        rcontext.RequestContext
	statements *mediaAttributesStoreStatements // copied from factory
}

func InitMediaAttributesStore(sqlDb *sql.DB) (*MediaAttributesStoreFactory, error) {
	store := MediaAttributesStoreFactory{stmts: &mediaAttributesStoreStatements{}}
	var err error

	store.sqlDb = sqlDb

	if store.stmts.selectMediaAttributes, err = store.sqlDb.Prepare(selectMediaAttributes); err != nil {
		return nil, err
	}
	if store.stmts.upsertMediaPurpose, err = store.sqlDb.Prepare(upsertMediaPurpose); err != nil {
		return nil, err
	}

	return &store, nil
}

func (f *MediaAttributesStoreFactory) Create(ctx rcontext.RequestContext) *MediaAttributesStore {
	return &MediaAttributesStore{
		factory:    f,
		ctx:        ctx,
		statements: f.stmts, // we copy this intentionally
	}
}

func (s *MediaAttributesStore) GetAttributes(origin string, mediaId string) (*types.MediaAttributes, error) {
	r := s.statements.selectMediaAttributes.QueryRowContext(s.ctx, origin, mediaId)
	obj := &types.MediaAttributes{}
	err := r.Scan(
		&obj.Origin,
		&obj.MediaId,
		&obj.Purpose,
	)
	return obj, err
}

func (s *MediaAttributesStore) GetAttributesDefaulted(origin string, mediaId string) (*types.MediaAttributes, error) {
	attr, err := s.GetAttributes(origin, mediaId)
	if err == sql.ErrNoRows {
		return &types.MediaAttributes{
			Origin:  origin,
			MediaId: mediaId,
			Purpose: types.PurposeNone,
		}, nil
	}
	return attr, err
}

func (s *MediaAttributesStore) UpsertPurpose(origin string, mediaId string, purpose string) error {
	_, err := s.statements.upsertMediaPurpose.ExecContext(s.ctx, origin, mediaId, purpose)
	return err
}
