package database

import (
	"database/sql"
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type DbTask struct {
	TaskId  int
	Name    string
	Params  *AnonymousJson
	StartTs int64
	EndTs   int64
}

const selectTask = "SELECT id, task, params, start_ts, end_ts FROM background_tasks WHERE id = $1;"
const insertTask = "INSERT INTO background_tasks (task, params, start_ts, end_ts) VALUES ($1, $2, $3, 0) RETURNING id, task, params, start_ts, end_ts;"

type tasksTableStatements struct {
	selectTask *sql.Stmt
	insertTask *sql.Stmt
}

type tasksTableWithContext struct {
	statements *tasksTableStatements
	ctx        rcontext.RequestContext
}

func prepareTasksTables(db *sql.DB) (*tasksTableStatements, error) {
	var err error
	var stmts = &tasksTableStatements{}

	if stmts.selectTask, err = db.Prepare(selectTask); err != nil {
		return nil, errors.New("error preparing selectTask: " + err.Error())
	}
	if stmts.insertTask, err = db.Prepare(insertTask); err != nil {
		return nil, errors.New("error preparing insertTask: " + err.Error())
	}

	return stmts, nil
}

func (s *tasksTableStatements) Prepare(ctx rcontext.RequestContext) *tasksTableWithContext {
	return &tasksTableWithContext{
		statements: s,
		ctx:        ctx,
	}
}

func (s *tasksTableWithContext) Insert(name string, params *AnonymousJson, startTs int64) (*DbTask, error) {
	row := s.statements.insertTask.QueryRowContext(s.ctx, name, params, startTs)
	val := &DbTask{}
	err := row.Scan(&val.TaskId, &val.Name, &val.Params, &val.StartTs, &val.EndTs)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (s *tasksTableWithContext) Get(id int) (*DbTask, error) {
	row := s.statements.selectTask.QueryRowContext(s.ctx, id)
	val := &DbTask{}
	err := row.Scan(&val.TaskId, &val.Name, &val.Params, &val.StartTs, &val.EndTs)
	if err == sql.ErrNoRows {
		err = nil
		val = nil
	}
	return val, err
}
