package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbBlurhash struct {
	Sha256Hash string
	Blurhash   string
}

const insertBlurhash = "INSERT INTO blurhashes (sha256_hash, blurhash) VALUES ($1, $2);"
const selectBlurhash = "SELECT blurhash FROM blurhashes WHERE sha256_hash = $1;"

type blurhashesTableStatements struct {
	insertBlurhash *sql.Stmt
	selectBlurhash *sql.Stmt
}

type blurhashesTableWithContext struct {
	statements *blurhashesTableStatements
	ctx        rcontext.RequestContext
}

func prepareBlurhashesTables(db *sql.DB) (*blurhashesTableStatements, error) {
	var err error
	var stmts = &blurhashesTableStatements{}

	if stmts.insertBlurhash, err = db.Prepare(insertBlurhash); err != nil {
		return nil, errors.New("error preparing insertBlurhash: " + err.Error())
	}
	if stmts.selectBlurhash, err = db.Prepare(selectBlurhash); err != nil {
		return nil, errors.New("error preparing selectBlurhash: " + err.Error())
	}

	return stmts, nil
}

func (s *blurhashesTableStatements) Prepare(ctx rcontext.RequestContext) *blurhashesTableWithContext {
	return &blurhashesTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *blurhashesTableWithContext) Insert(sha256hash string, blurhash string) error {
	_, err := s.statements.insertBlurhash.ExecContext(s.ctx, sha256hash, blurhash)
	return err
}

func (s *blurhashesTableWithContext) Get(hash string) (string, error) {
	row := s.statements.selectBlurhash.QueryRowContext(s.ctx, hash)
	val := ""
	err := row.Scan(&val)
	if err == sql.ErrNoRows {
		err = nil
		val = ""
	}
	return val, err
}
