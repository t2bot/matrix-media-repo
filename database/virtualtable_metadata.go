package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

const selectEstimatedDatastoreSize = "SELECT COALESCE(SUM(m2.size_bytes), 0) + COALESCE((SELECT SUM(t2.size_bytes) FROM (SELECT DISTINCT t.sha256_hash, MAX(t.size_bytes) AS size_bytes FROM thumbnails AS t WHERE t.datastore_id = $1 GROUP BY t.sha256_hash) AS t2), 0) AS size_total FROM (SELECT DISTINCT m.sha256_hash, MAX(m.size_bytes) AS size_bytes FROM media AS m WHERE m.datastore_id = $1 GROUP BY m.sha256_hash) AS m2;"

type metadataVirtualTableStatements struct {
	selectEstimatedDatastoreSize *sql.Stmt
}

type metadataVirtualTableWithContext struct {
	statements *metadataVirtualTableStatements
	ctx        rcontext.RequestContext
}

func prepareMetadataVirtualTables(db *sql.DB) (*metadataVirtualTableStatements, error) {
	var err error
	var stmts = &metadataVirtualTableStatements{}

	if stmts.selectEstimatedDatastoreSize, err = db.Prepare(selectEstimatedDatastoreSize); err != nil {
		return nil, errors.New("error preparing selectEstimatedDatastoreSize: " + err.Error())
	}

	return stmts, nil
}

func (s *metadataVirtualTableStatements) Prepare(ctx rcontext.RequestContext) *metadataVirtualTableWithContext {
	return &metadataVirtualTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *metadataVirtualTableWithContext) EstimateDatastoreSize(datastoreId string) (int64, error) {
	row := s.statements.selectEstimatedDatastoreSize.QueryRowContext(s.ctx, datastoreId)
	val := int64(0)
	err := row.Scan(&val)
	if err == sql.ErrNoRows {
		err = nil
		val = 0
	}
	return val, err
}
