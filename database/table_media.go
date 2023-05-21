package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbMedia struct {
	Origin      string
	MediaId     string
	UploadName  string
	ContentType string
	UserId      string
	Sha256Hash  string
	SizeBytes   int64
	CreationTs  int64
	Quarantined bool
	DatastoreId string
	Location    string
}

const selectDistinctMediaDatastoreIds = "SELECT DISTINCT datastore_id FROM media;"

type mediaTableStatements struct {
	selectDistinctMediaDatastoreIds *sql.Stmt
}

type mediaTableWithContext struct {
	statements *mediaTableStatements
	ctx        rcontext.RequestContext
}

func prepareMediaTables(db *sql.DB) (*mediaTableStatements, error) {
	var err error
	var stmts = &mediaTableStatements{}

	if stmts.selectDistinctMediaDatastoreIds, err = db.Prepare(selectDistinctMediaDatastoreIds); err != nil {
		return nil, errors.New("error preparing selectDistinctMediaDatastoreIds: " + err.Error())
	}

	return stmts, nil
}

func (s *mediaTableStatements) Prepare(ctx rcontext.RequestContext) *mediaTableWithContext {
	return &mediaTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *mediaTableWithContext) GetDistinctDatastoreIds() ([]string, error) {
	rows, err := s.statements.selectDistinctMediaDatastoreIds.QueryContext(s.ctx)
	if err != nil {
		return nil, err
	}

	var results []string
	for rows.Next() {
		val := ""
		if err = rows.Scan(&val); err != nil {
			return nil, err
		}
		results = append(results, val)
	}

	return results, nil
}
