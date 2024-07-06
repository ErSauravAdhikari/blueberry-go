package store

import (
	"context"
	"errors"
	rasberry "github.com/ersauravadhikari/raspberry-go/raspberry"
	"sync"
)

type InMemoryDB struct {
	taskRuns         map[int]*rasberry.TaskRun
	taskRunLogs      map[int][]*rasberry.TaskRunLog
	mutex            sync.RWMutex
	nextTaskRunID    int
	nextTaskRunLogID int
}

func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		taskRuns:         make(map[int]*rasberry.TaskRun),
		taskRunLogs:      make(map[int][]*rasberry.TaskRunLog),
		nextTaskRunID:    1,
		nextTaskRunLogID: 1,
	}
}

func (db *InMemoryDB) SaveTaskRun(ctx context.Context, taskRun *rasberry.TaskRun) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if taskRun.ID == 0 {
		taskRun.ID = db.nextTaskRunID
		db.nextTaskRunID++
	}
	db.taskRuns[taskRun.ID] = taskRun
	return nil
}

func (db *InMemoryDB) SaveTaskRunLog(ctx context.Context, taskRunLog *rasberry.TaskRunLog) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	taskRunLog.ID = db.nextTaskRunLogID
	db.nextTaskRunLogID++
	db.taskRunLogs[taskRunLog.TaskRunID] = append(db.taskRunLogs[taskRunLog.TaskRunID], taskRunLog)
	return nil
}

func (db *InMemoryDB) GetTaskRuns(ctx context.Context) ([]rasberry.TaskRun, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	var taskRuns []rasberry.TaskRun
	for _, taskRun := range db.taskRuns {
		taskRuns = append(taskRuns, *taskRun)
	}
	return taskRuns, nil
}

func (db *InMemoryDB) GetTaskRunLogs(ctx context.Context, taskRunID int) ([]rasberry.TaskRunLog, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	taskRunLogs, exists := db.taskRunLogs[taskRunID]
	if !exists {
		return nil, errors.New("task run logs not found")
	}

	var result []rasberry.TaskRunLog
	for _, log := range taskRunLogs {
		result = append(result, *log)
	}
	return result, nil
}

func (db *InMemoryDB) GetPaginatedTaskRunLogs(ctx context.Context, taskRunID int, level string, page, size int) ([]rasberry.TaskRunLog, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	taskRunLogs, exists := db.taskRunLogs[taskRunID]
	if !exists {
		return nil, errors.New("task run logs not found")
	}

	var filteredLogs []rasberry.TaskRunLog
	for _, log := range taskRunLogs {
		if level == "all" || log.Level == level {
			filteredLogs = append(filteredLogs, *log)
		}
	}

	start := (page - 1) * size
	if start >= len(filteredLogs) {
		return nil, nil
	}
	end := start + size
	if end > len(filteredLogs) {
		end = len(filteredLogs)
	}

	return filteredLogs[start:end], nil
}

func (db *InMemoryDB) GetTaskRunByID(ctx context.Context, id int) (*rasberry.TaskRun, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	taskRun, exists := db.taskRuns[id]
	if !exists {
		return nil, errors.New("task run not found")
	}
	return taskRun, nil
}

func (db *InMemoryDB) Close() error {
	// No resources to close in an in-memory store
	return nil
}
