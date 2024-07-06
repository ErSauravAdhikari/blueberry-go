package store

import (
	"bufio"
	"context"
	"encoding/json"
	rasberry "github.com/ersauravadhikari/raspberry-go/raspberry"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type FileSystemDB struct {
	baseDir          string
	taskRunMutex     sync.RWMutex
	logMutex         sync.RWMutex
	nextTaskRunID    int
	nextTaskRunLogID int
}

func NewFileSystemDB(baseDir string) (*FileSystemDB, error) {
	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return nil, err
	}

	db := &FileSystemDB{
		baseDir:          baseDir,
		nextTaskRunID:    1,
		nextTaskRunLogID: 1,
	}

	// Initialize nextTaskRunID
	taskRunDir := filepath.Join(baseDir, "task_runs")
	if err := os.MkdirAll(taskRunDir, os.ModePerm); err != nil {
		return nil, err
	}

	taskRunFiles, err := os.ReadDir(taskRunDir)
	if err != nil {
		return nil, err
	}

	for _, file := range taskRunFiles {
		if !file.IsDir() {
			fileID, err := strconv.Atoi(file.Name()[:len(file.Name())-len(filepath.Ext(file.Name()))])
			if err == nil && fileID >= db.nextTaskRunID {
				db.nextTaskRunID = fileID + 1
			}
		}
	}

	// Initialize nextTaskRunLogID
	logDir := filepath.Join(baseDir, "task_run_logs")
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil, err
	}

	logFiles, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	for _, file := range logFiles {
		if !file.IsDir() {
			fileID, err := strconv.Atoi(file.Name()[:len(file.Name())-len(filepath.Ext(file.Name()))])
			if err == nil && fileID >= db.nextTaskRunLogID {
				db.nextTaskRunLogID = fileID + 1
			}
		}
	}

	return db, nil
}

func (db *FileSystemDB) SaveTaskRun(ctx context.Context, taskRun *rasberry.TaskRun) error {
	db.taskRunMutex.Lock()
	defer db.taskRunMutex.Unlock()

	if taskRun.ID == 0 {
		taskRun.ID = db.nextTaskRunID
		db.nextTaskRunID++
	}
	data, err := json.Marshal(taskRun)
	if err != nil {
		return err
	}

	taskRunDir := filepath.Join(db.baseDir, "task_runs")
	if err := os.MkdirAll(taskRunDir, os.ModePerm); err != nil {
		return err
	}

	taskRunFile := filepath.Join(taskRunDir, strconv.Itoa(taskRun.ID)+".json")
	return os.WriteFile(taskRunFile, data, 0644)
}

func (db *FileSystemDB) SaveTaskRunLog(ctx context.Context, taskRunLog *rasberry.TaskRunLog) error {
	db.logMutex.Lock()
	defer db.logMutex.Unlock()

	taskRunLog.ID = db.nextTaskRunLogID
	db.nextTaskRunLogID++
	data, err := json.Marshal(taskRunLog)
	if err != nil {
		return err
	}

	logDir := filepath.Join(db.baseDir, "task_run_logs")
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return err
	}

	logFile := filepath.Join(logDir, strconv.Itoa(taskRunLog.TaskRunID)+".jsonl")
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(string(data) + "\n")
	return err
}

func (db *FileSystemDB) GetTaskRuns(ctx context.Context) ([]rasberry.TaskRun, error) {
	db.taskRunMutex.RLock()
	defer db.taskRunMutex.RUnlock()

	taskRunDir := filepath.Join(db.baseDir, "task_runs")
	if _, err := os.Stat(taskRunDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(taskRunDir)
	if err != nil {
		return nil, err
	}

	var taskRuns []rasberry.TaskRun
	for _, file := range files {
		if !file.IsDir() {
			data, err := os.ReadFile(filepath.Join(taskRunDir, file.Name()))
			if err != nil {
				return nil, err
			}

			var taskRun rasberry.TaskRun
			if err := json.Unmarshal(data, &taskRun); err != nil {
				return nil, err
			}
			taskRuns = append(taskRuns, taskRun)
		}
	}
	return taskRuns, nil
}

func (db *FileSystemDB) GetTaskRunLogs(ctx context.Context, taskRunID int) ([]rasberry.TaskRunLog, error) {
	db.logMutex.RLock()
	defer db.logMutex.RUnlock()

	logFile := filepath.Join(db.baseDir, "task_run_logs", strconv.Itoa(taskRunID)+".jsonl")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var taskRunLogs []rasberry.TaskRunLog
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var taskRunLog rasberry.TaskRunLog
		if err := json.Unmarshal(scanner.Bytes(), &taskRunLog); err != nil {
			return nil, err
		}
		taskRunLogs = append(taskRunLogs, taskRunLog)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return taskRunLogs, nil
}

func (db *FileSystemDB) GetPaginatedTaskRunLogs(ctx context.Context, taskRunID int, level string, page, size int) ([]rasberry.TaskRunLog, error) {
	allLogs, err := db.GetTaskRunLogs(ctx, taskRunID)
	if err != nil {
		return nil, err
	}

	var filteredLogs []rasberry.TaskRunLog
	for _, log := range allLogs {
		if level == "all" || log.Level == level {
			filteredLogs = append(filteredLogs, log)
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

func (db *FileSystemDB) GetTaskRunByID(ctx context.Context, id int) (*rasberry.TaskRun, error) {
	db.taskRunMutex.RLock()
	defer db.taskRunMutex.RUnlock()

	taskRunFile := filepath.Join(db.baseDir, "task_runs", strconv.Itoa(id)+".json")
	if _, err := os.Stat(taskRunFile); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(taskRunFile)
	if err != nil {
		return nil, err
	}

	var taskRun rasberry.TaskRun
	if err := json.Unmarshal(data, &taskRun); err != nil {
		return nil, err
	}
	return &taskRun, nil
}

func (db *FileSystemDB) Close() error {
	// No resources to close in a filesystem store
	return nil
}
