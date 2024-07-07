package store

import (
	"context"
	"encoding/json"
	rasberry "github.com/ersauravadhikari/raspberry-go/raspberry"
	"github.com/jackc/pgx/v4"
)

type PostgresDB struct {
	conn *pgx.Conn
}

func NewPostgresDB(connStr string) (*PostgresDB, error) {
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return nil, err
	}

	db := &PostgresDB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *PostgresDB) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS task_runs (
		id SERIAL PRIMARY KEY,
		task_name VARCHAR(255),
		start_time TIMESTAMP,
		end_time TIMESTAMP,
		params JSONB,
		status VARCHAR(50)
	);

	CREATE TABLE IF NOT EXISTS task_run_logs (
		id SERIAL PRIMARY KEY,
		task_run_id INTEGER,
		timestamp TIMESTAMP,
		level VARCHAR(50),
		message TEXT,
		FOREIGN KEY (task_run_id) REFERENCES task_runs(id)
	);
	`

	_, err := db.conn.Exec(context.Background(), query)
	return err
}

func (db *PostgresDB) SaveTaskRun(ctx context.Context, taskRun *rasberry.TaskRun) error {
	params, _ := json.Marshal(taskRun.Params)
	if taskRun.ID == 0 {
		return db.conn.QueryRow(ctx,
			"INSERT INTO task_runs (task_name, start_time, end_time, params, status) VALUES ($1, $2, $3, $4, $5) RETURNING id",
			taskRun.TaskName, taskRun.StartTime, taskRun.EndTime, params, taskRun.Status).Scan(&taskRun.ID)
	} else {
		_, err := db.conn.Exec(ctx,
			"UPDATE task_runs SET task_name = $1, start_time = $2, end_time = $3, params = $4, status = $5 WHERE id = $6",
			taskRun.TaskName, taskRun.StartTime, taskRun.EndTime, params, taskRun.Status, taskRun.ID)
		return err
	}
}

func (db *PostgresDB) SaveTaskRunLog(ctx context.Context, taskRunLog *rasberry.TaskRunLog) error {
	return db.conn.QueryRow(ctx,
		"INSERT INTO task_run_logs (task_run_id, timestamp, level, message) VALUES ($1, $2, $3, $4) RETURNING id",
		taskRunLog.TaskRunID, taskRunLog.Timestamp, taskRunLog.Level, taskRunLog.Message).Scan(&taskRunLog.ID)
}

func (db *PostgresDB) GetTaskRuns(ctx context.Context) ([]rasberry.TaskRun, error) {
	rows, err := db.conn.Query(ctx, "SELECT id, task_name, start_time, end_time, params, status FROM task_runs")
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
	return taskRuns, nil
}

func (db *PostgresDB) GetPaginatedTaskRunsForTaskName(ctx context.Context, name string, page, limit int) ([]rasberry.TaskRun, error) {
	offset := (page - 1) * limit
	rows, err := db.conn.Query(ctx, "SELECT id, task_name, start_time, end_time, params, status FROM task_runs WHERE task_name = $1 ORDER BY start_time DESC LIMIT $2 OFFSET $3", name, limit, offset)
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
	return taskRuns, nil
}

func (db *PostgresDB) GetTaskRunsCountForTaskName(ctx context.Context, name string) (int, error) {
	var count int
	err := db.conn.QueryRow(ctx, "SELECT COUNT(*) FROM task_runs WHERE task_name = $1", name).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (db *PostgresDB) GetTaskRunLogs(ctx context.Context, taskRunID int) ([]rasberry.TaskRunLog, error) {
	rows, err := db.conn.Query(ctx, "SELECT id, task_run_id, timestamp, level, message FROM task_run_logs WHERE task_run_id = $1", taskRunID)
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
	return taskRunLogs, nil
}
func (db *PostgresDB) GetPaginatedTaskRunLogs(ctx context.Context, taskRunID int, level string, page, size int) ([]rasberry.TaskRunLog, error) {
	query := "SELECT id, task_run_id, timestamp, level, message FROM task_run_logs WHERE task_run_id = $1"
	args := []interface{}{taskRunID}
	if level != "all" {
		query += " AND level = $2"
		args = append(args, level)
	}
	query += " LIMIT $3 OFFSET $4"
	args = append(args, size, (page-1)*size)

	rows, err := db.conn.Query(ctx, query, args...)
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
	return taskRunLogs, nil
}

func (db *PostgresDB) GetTaskRunByID(ctx context.Context, id int) (*rasberry.TaskRun, error) {
	row := db.conn.QueryRow(ctx, "SELECT id, task_name, start_time, end_time, params, status FROM task_runs WHERE id = $1", id)
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

func (db *PostgresDB) Close() error {
	return db.conn.Close(context.Background())
}
