package database

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbDatastore struct {
	DatastoreId string
	Type        string
	Uri         string
}

const selectAllDatastores = "SELECT datastore_id, ds_type, uri FROM datastores;"
const selectDatastore = "SELECT datastore_id, ds_type, uri FROM datastores WHERE datastore_id = $1;"
const selectDatastoreByUri = "SELECT datastore_id, ds_type, uri FROM datastores WHERE uri = $1;"
const insertDatastore = "INSERT INTO datastores (datastore_id, ds_type, uri) VALUES ($1, $2, $3);"

var dsCacheByPath = sync.Map{} // [string] => Datastore
var dsCacheById = sync.Map{}   // [string] => Datastore

type dsTableStatements struct {
	selectAllDatastores  *sql.Stmt
	selectDatastore      *sql.Stmt
	selectDatastoreByUri *sql.Stmt
	insertDatastore      *sql.Stmt
}

type dsTableWithContext struct {
	statements *dsTableStatements
	ctx        rcontext.RequestContext
}

func prepareDatastoreTables(db *sql.DB) (*dsTableStatements, error) {
	var err error
	var stmts = &dsTableStatements{}

	if stmts.selectAllDatastores, err = db.Prepare(selectAllDatastores); err != nil {
		return nil, errors.New("error preparing selectAllDatastores: " + err.Error())
	}
	if stmts.selectDatastore, err = db.Prepare(selectDatastore); err != nil {
		return nil, errors.New("error preparing selectDatastore: " + err.Error())
	}
	if stmts.selectDatastoreByUri, err = db.Prepare(selectDatastoreByUri); err != nil {
		return nil, errors.New("error preparing selectDatastoreByUri: " + err.Error())
	}
	if stmts.insertDatastore, err = db.Prepare(insertDatastore); err != nil {
		return nil, errors.New("error preparing insertDatastore: " + err.Error())
	}

	return stmts, nil
}

func (s *dsTableStatements) Prepare(ctx rcontext.RequestContext) *dsTableWithContext {
	return &dsTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *dsTableWithContext) GetDatastore(id string) (*DbDatastore, error) {
	if v, ok := dsCacheById.Load(id); ok {
		ds := v.(*DbDatastore)
		return &DbDatastore{
			DatastoreId: ds.DatastoreId,
			Type:        ds.Type,
			Uri:         ds.Uri,
		}, nil
	}

	d := &DbDatastore{}
	if err := s.statements.selectDatastore.QueryRowContext(s.ctx, id).Scan(&d.DatastoreId, &d.Type, &d.Uri); err != nil {
		return nil, err
	}

	dsCacheById.Store(d.DatastoreId, d)
	dsCacheByPath.Store(d.Uri, d)

	return d, nil
}

func (s *dsTableWithContext) GetDatastoreByUri(uri string) (*DbDatastore, error) {
	if v, ok := dsCacheByPath.Load(uri); ok {
		ds := v.(*DbDatastore)
		return &DbDatastore{
			DatastoreId: ds.DatastoreId,
			Type:        ds.Type,
			Uri:         ds.Uri,
		}, nil
	}

	d := &DbDatastore{}
	if err := s.statements.selectDatastoreByUri.QueryRowContext(s.ctx, uri).Scan(&d.DatastoreId, &d.Type, &d.Uri); err != nil {
		return nil, err
	}

	dsCacheById.Store(d.DatastoreId, d)
	dsCacheByPath.Store(d.Uri, d)

	return d, nil
}

func (s *dsTableWithContext) GetAllDatastores() ([]*DbDatastore, error) {
	rows, err := s.statements.selectAllDatastores.QueryContext(s.ctx)
	if err != nil {
		return nil, err
	}

	var results []*DbDatastore
	for rows.Next() {
		obj := &DbDatastore{}
		if err = rows.Scan(&obj.DatastoreId, &obj.Type, &obj.Uri); err != nil {
			return nil, err
		}
	}

	return results, nil
}

func (s *dsTableWithContext) InsertDatastore(ds *DbDatastore) error {
	if _, err := s.statements.insertDatastore.ExecContext(s.ctx, ds.DatastoreId, ds.Type, ds.Uri); err != nil {
		return errors.New("error persiting datastore record: " + err.Error())
	}

	dsCacheById.Store(ds.DatastoreId, ds)
	dsCacheByPath.Store(ds.Uri, ds)

	return nil
}
