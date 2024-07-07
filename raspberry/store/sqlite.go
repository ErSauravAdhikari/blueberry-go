package store

import (
	"context"
	"database/sql"
	"encoding/json"
	rasberry "github.com/ersauravadhikari/raspberry-go/raspberry"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDB struct {
	conn *sql.DB
}

func NewSQLiteDB(connStr string) (*SQLiteDB, error) {
	conn, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	db := &SQLiteDB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *SQLiteDB) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS task_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_name TEXT,
		start_time TIMESTAMP,
		end_time TIMESTAMP,
		params TEXT,
		status TEXT
	);

	CREATE TABLE IF NOT EXISTS task_run_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_run_id INTEGER,
		timestamp TIMESTAMP,
		level TEXT,
		message TEXT,
		FOREIGN KEY (task_run_id) REFERENCES task_runs(id)
	);
	`

	_, err := db.conn.Exec(query)
	return err
}

func (db *SQLiteDB) SaveTaskRun(ctx context.Context, taskRun *rasberry.TaskRun) error {
	params, _ := json.Marshal(taskRun.Params)
	if taskRun.ID == 0 {
		result, err := db.conn.ExecContext(ctx,
			"INSERT INTO task_runs (task_name, start_time, end_time, params, status) VALUES (?, ?, ?, ?, ?)",
			taskRun.TaskName, taskRun.StartTime, taskRun.EndTime, params, taskRun.Status)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		taskRun.ID = int(id)
	} else {
		_, err := db.conn.ExecContext(ctx,
			"UPDATE task_runs SET task_name = ?, start_time = ?, end_time = ?, params = ?, status = ? WHERE id = ?",
			taskRun.TaskName, taskRun.StartTime, taskRun.EndTime, params, taskRun.Status, taskRun.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLiteDB) GetTaskRunByID(ctx context.Context, id int) (*rasberry.TaskRun, error) {
	row := db.conn.QueryRowContext(ctx, "SELECT id, task_name, start_time, end_time, params, status FROM task_runs WHERE id = ?", id)
	var taskRun rasberry.TaskRun
	var params []byte
	if err := row.Scan(&taskRun.ID, &taskRun.TaskName, &taskRun.StartTime, &taskRun.EndTime, &params, &taskRun.Status); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(params, &taskRun.Params); err != nil {
		return nil, err
	}
	return &taskRun, nil
}

func (db *SQLiteDB) SaveTaskRunLog(ctx context.Context, taskRunLog *rasberry.TaskRunLog) error {
	result, err := db.conn.ExecContext(ctx,
		"INSERT INTO task_run_logs (task_run_id, timestamp, level, message) VALUES (?, ?, ?, ?)",
		taskRunLog.TaskRunID, taskRunLog.Timestamp, taskRunLog.Level, taskRunLog.Message)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	taskRunLog.ID = int(id)
	return nil
}

func (db *SQLiteDB) GetTaskRuns(ctx context.Context) ([]rasberry.TaskRun, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id, task_name, start_time, end_time, params, status FROM task_runs ORDER BY start_time DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var taskRuns []rasberry.TaskRun
	for rows.Next() {
		var taskRun rasberry.TaskRun
		var params []byte
		if err := rows.Scan(&taskRun.ID, &taskRun.TaskName, &taskRun.StartTime, &taskRun.EndTime, &params, &taskRun.Status); err != nil {
			return nil, err
		}
		json.Unmarshal(params, &taskRun.Params)
		taskRuns = append(taskRuns, taskRun)
	}

	if taskRuns == nil {
		return []rasberry.TaskRun{}, nil
	}

	return taskRuns, nil
}

func (db *SQLiteDB) GetPaginatedTaskRunsForTaskName(ctx context.Context, name string, page, limit int) ([]rasberry.TaskRun, error) {
	offset := (page - 1) * limit
	rows, err := db.conn.QueryContext(ctx, "SELECT id, task_name, start_time, end_time, params, status FROM task_runs WHERE task_name = ? ORDER BY start_time DESC LIMIT ? OFFSET ?", name, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var taskRuns []rasberry.TaskRun
	for rows.Next() {
		var taskRun rasberry.TaskRun
		var params []byte
		if err := rows.Scan(&taskRun.ID, &taskRun.TaskName, &taskRun.StartTime, &taskRun.EndTime, &params, &taskRun.Status); err != nil {
			return nil, err
		}
		json.Unmarshal(params, &taskRun.Params)
		taskRuns = append(taskRuns, taskRun)
	}

	if taskRuns == nil {
		return []rasberry.TaskRun{}, nil
	}

	return taskRuns, nil
}

func (db *SQLiteDB) GetTaskRunsCountForTaskName(ctx context.Context, name string) (int, error) {
	var count int
	err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM task_runs WHERE task_name = ?", name).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (db *SQLiteDB) GetTaskRunLogs(ctx context.Context, taskRunID int) ([]rasberry.TaskRunLog, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id, task_run_id, timestamp, level, message FROM task_run_logs WHERE task_run_id = ?", taskRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var taskRunLogs []rasberry.TaskRunLog
	for rows.Next() {
		var taskRunLog rasberry.TaskRunLog
		if err := rows.Scan(&taskRunLog.ID, &taskRunLog.TaskRunID, &taskRunLog.Timestamp, &taskRunLog.Level, &taskRunLog.Message); err != nil {
			return nil, err
		}
		taskRunLogs = append(taskRunLogs, taskRunLog)
	}

	if taskRunLogs == nil {
		return []rasberry.TaskRunLog{}, nil
	}

	return taskRunLogs, nil
}

func (db *SQLiteDB) GetPaginatedTaskRunLogs(ctx context.Context, taskRunID int, level string, page, size int) ([]rasberry.TaskRunLog, error) {
	query := "SELECT id, task_run_id, timestamp, level, message FROM task_run_logs WHERE task_run_id = ?"
	args := []interface{}{taskRunID}
	if level != "all" {
		query += " AND level = ?"
		args = append(args, level)
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, size, (page-1)*size)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var taskRunLogs []rasberry.TaskRunLog
	for rows.Next() {
		var taskRunLog rasberry.TaskRunLog
		if err := rows.Scan(&taskRunLog.ID, &taskRunLog.TaskRunID, &taskRunLog.Timestamp, &taskRunLog.Level, &taskRunLog.Message); err != nil {
			return nil, err
		}
		taskRunLogs = append(taskRunLogs, taskRunLog)
	}

	if taskRunLogs == nil {
		return []rasberry.TaskRunLog{}, nil
	}

	return taskRunLogs, nil
}

func (db *SQLiteDB) Close() error {
	return db.conn.Close()
}
