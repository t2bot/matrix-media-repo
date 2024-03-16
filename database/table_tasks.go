package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type DbTask struct {
	TaskId  int
	Name    string
	Params  *AnonymousJson
	StartTs int64
	EndTs   int64
	Error   string
}

const selectTask = "SELECT id, task, params, start_ts, end_ts, error FROM background_tasks WHERE id = $1;"
const insertTask = "INSERT INTO background_tasks (task, params, start_ts, end_ts) VALUES ($1, $2, $3, 0) RETURNING id, task, params, start_ts, end_ts, error;"
const selectAllTasks = "SELECT id, task, params, start_ts, end_ts, error FROM background_tasks;"
const selectIncompleteTasks = "SELECT id, task, params, start_ts, end_ts, error FROM background_tasks WHERE end_ts <= 0;"
const updateTaskEndTime = "UPDATE background_tasks SET end_ts = $2 WHERE id = $1;"
const updateTaskError = "UPDATE background_tasks SET error = $2 WHERE id = $1;"

type tasksTableStatements struct {
	selectTask            *sql.Stmt
	insertTask            *sql.Stmt
	selectAllTasks        *sql.Stmt
	selectIncompleteTasks *sql.Stmt
	updateTaskEndTime     *sql.Stmt
	updateTaskError       *sql.Stmt
}

type tasksTableWithContext struct {
	statements *tasksTableStatements
	ctx        rcontext.RequestContext
}

func prepareTasksTables(db *sql.DB) (*tasksTableStatements, error) {
	var err error
	stmts := &tasksTableStatements{}

	if stmts.selectTask, err = db.Prepare(selectTask); err != nil {
		return nil, fmt.Errorf("error preparing selectTask: %w", err)
	}
	if stmts.insertTask, err = db.Prepare(insertTask); err != nil {
		return nil, fmt.Errorf("error preparing insertTask: %w", err)
	}
	if stmts.selectAllTasks, err = db.Prepare(selectAllTasks); err != nil {
		return nil, fmt.Errorf("error preparing selectAllTasks: %w", err)
	}
	if stmts.selectIncompleteTasks, err = db.Prepare(selectIncompleteTasks); err != nil {
		return nil, fmt.Errorf("error preparing selectIncompleteTasks: %w", err)
	}
	if stmts.updateTaskEndTime, err = db.Prepare(updateTaskEndTime); err != nil {
		return nil, fmt.Errorf("error preparing updateTaskEndTime: %w", err)
	}
	if stmts.updateTaskError, err = db.Prepare(updateTaskError); err != nil {
		return nil, fmt.Errorf("error preparing updateTaskError: %w", err)
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
	err := row.Scan(&val.TaskId, &val.Name, &val.Params, &val.StartTs, &val.EndTs, &val.Error)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (s *tasksTableWithContext) SetEndTime(taskId int, endTs int64) error {
	_, err := s.statements.updateTaskEndTime.ExecContext(s.ctx, taskId, endTs)
	return err
}

func (s *tasksTableWithContext) SetError(taskId int, errVal string) error {
	_, err := s.statements.updateTaskError.ExecContext(s.ctx, taskId, errVal)
	return err
}

func (s *tasksTableWithContext) Get(id int) (*DbTask, error) {
	row := s.statements.selectTask.QueryRowContext(s.ctx, id)
	val := &DbTask{}
	err := row.Scan(&val.TaskId, &val.Name, &val.Params, &val.StartTs, &val.EndTs, &val.Error)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		val = nil
	}
	return val, err
}

func (s *tasksTableWithContext) GetAll(includingFinished bool) ([]*DbTask, error) {
	results := make([]*DbTask, 0)
	q := s.statements.selectAllTasks
	if !includingFinished {
		q = s.statements.selectIncompleteTasks
	}
	rows, err := q.QueryContext(s.ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return results, nil
		}
		return nil, err
	}
	for rows.Next() {
		val := &DbTask{}
		if err = rows.Scan(&val.TaskId, &val.Name, &val.Params, &val.StartTs, &val.EndTs, &val.Error); err != nil {
			return nil, err
		}
		results = append(results, val)
	}
	return results, nil
}
