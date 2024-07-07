package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ersauravadhikari/blueberry-go/blueberry"
	"os"
	"path/filepath"
	"sync"
)

type Metadata struct {
	LastTaskID    int              `json:"last_task_id"`
	TaskNameToIDs map[string][]int `json:"task_name_to_ids"`
}

type FileStoreDB struct {
	baseDir  string
	mu       sync.Mutex
	metadata Metadata
}

func NewFileStoreDB(baseDir string) (*FileStoreDB, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}

	db := &FileStoreDB{
		baseDir: baseDir,
		metadata: Metadata{
			TaskNameToIDs: make(map[string][]int),
		},
	}

	if err := db.loadMetadata(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *FileStoreDB) loadMetadata() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	metadataFilePath := filepath.Join(db.baseDir, "metadata.json")
	if _, err := os.Stat(metadataFilePath); os.IsNotExist(err) {
		return nil // No metadata file exists yet
	}

	f, err := os.Open(metadataFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	return decoder.Decode(&db.metadata)
}

func (db *FileStoreDB) saveMetadata() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	metadataFilePath := filepath.Join(db.baseDir, "metadata.json")
	f, err := os.Create(metadataFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	return encoder.Encode(&db.metadata)
}

func (db *FileStoreDB) SaveTaskRun(ctx context.Context, taskRun *blueberry.TaskRun) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if taskRun.ID == 0 {
		db.metadata.LastTaskID++
		taskRun.ID = db.metadata.LastTaskID
	}

	taskDir := filepath.Join(db.baseDir, taskRun.TaskName)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(taskDir, fmt.Sprintf("task_%d.json", taskRun.ID))
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	if err := encoder.Encode(taskRun); err != nil {
		return err
	}

	db.metadata.TaskNameToIDs[taskRun.TaskName] = append(db.metadata.TaskNameToIDs[taskRun.TaskName], taskRun.ID)
	return db.saveMetadata()
}

func (db *FileStoreDB) SaveTaskRunLog(ctx context.Context, taskRunLog *blueberry.TaskRunLog) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	taskDir := filepath.Join(db.baseDir, fmt.Sprintf("task_%d_logs", taskRunLog.TaskRunID))
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return err
	}

	logFilePath := filepath.Join(taskDir, "logs.jsonl")
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	logEntry, err := json.Marshal(taskRunLog)
	if err != nil {
		return err
	}

	_, err = f.WriteString(string(logEntry) + "\n")
	return err
}

func (db *FileStoreDB) GetTaskRuns(ctx context.Context) ([]blueberry.TaskRun, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var taskRuns []blueberry.TaskRun
	for taskName, ids := range db.metadata.TaskNameToIDs {
		taskDir := filepath.Join(db.baseDir, taskName)
		for _, id := range ids {
			filePath := filepath.Join(taskDir, fmt.Sprintf("task_%d.json", id))
			f, err := os.Open(filePath)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			var taskRun blueberry.TaskRun
			decoder := json.NewDecoder(f)
			if err := decoder.Decode(&taskRun); err != nil {
				return nil, err
			}
			taskRuns = append(taskRuns, taskRun)
		}
	}

	return taskRuns, nil
}

func (db *FileStoreDB) GetTaskRunByID(ctx context.Context, id int) (*blueberry.TaskRun, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	for taskName, ids := range db.metadata.TaskNameToIDs {
		for _, taskID := range ids {
			if taskID == id {
				filePath := filepath.Join(db.baseDir, taskName, fmt.Sprintf("task_%d.json", id))
				f, err := os.Open(filePath)
				if err != nil {
					return nil, err
				}
				defer f.Close()

				var taskRun blueberry.TaskRun
				decoder := json.NewDecoder(f)
				if err := decoder.Decode(&taskRun); err != nil {
					return nil, err
				}

				return &taskRun, nil
			}
		}
	}

	return nil, fmt.Errorf("task run with ID %d not found", id)
}

func (db *FileStoreDB) GetTaskRunLogs(ctx context.Context, taskRunID int) ([]blueberry.TaskRunLog, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	logFilePath := filepath.Join(db.baseDir, fmt.Sprintf("task_%d_logs", taskRunID), "logs.jsonl")
	f, err := os.Open(logFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var taskRunLogs []blueberry.TaskRunLog
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var logEntry blueberry.TaskRunLog
		if err := json.Unmarshal(scanner.Bytes(), &logEntry); err != nil {
			return nil, err
		}
		taskRunLogs = append(taskRunLogs, logEntry)
	}

	return taskRunLogs, scanner.Err()
}

func (db *FileStoreDB) GetPaginatedTaskRunLogs(ctx context.Context, taskRunID int, level string, page, size int) ([]blueberry.TaskRunLog, error) {
	allLogs, err := db.GetTaskRunLogs(ctx, taskRunID)
	if err != nil {
		return nil, err
	}

	var filteredLogs []blueberry.TaskRunLog
	for _, log := range allLogs {
		if level == "all" || log.Level == level {
			filteredLogs = append(filteredLogs, log)
		}
	}

	start := (page - 1) * size
	end := start + size
	if start > len(filteredLogs) {
		return []blueberry.TaskRunLog{}, nil
	}
	if end > len(filteredLogs) {
		end = len(filteredLogs)
	}

	return filteredLogs[start:end], nil
}

func (db *FileStoreDB) GetPaginatedTaskRunsForTaskName(ctx context.Context, name string, page, limit int) ([]blueberry.TaskRun, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	ids, exists := db.metadata.TaskNameToIDs[name]
	if !exists {
		return []blueberry.TaskRun{}, nil
	}

	start := (page - 1) * limit
	end := start + limit
	if start > len(ids) {
		return []blueberry.TaskRun{}, nil
	}
	if end > len(ids) {
		end = len(ids)
	}

	var taskRuns []blueberry.TaskRun
	for _, id := range ids[start:end] {
		filePath := filepath.Join(db.baseDir, name, fmt.Sprintf("task_%d.json", id))
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		var taskRun blueberry.TaskRun
		decoder := json.NewDecoder(f)
		if err := decoder.Decode(&taskRun); err != nil {
			return nil, err
		}
		taskRuns = append(taskRuns, taskRun)
	}

	return taskRuns, nil
}

func (db *FileStoreDB) GetTaskRunsCountForTaskName(ctx context.Context, name string) (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	ids, exists := db.metadata.TaskNameToIDs[name]
	if !exists {
		return 0, nil
	}

	return len(ids), nil
}

func (db *FileStoreDB) Close() error {
	return db.saveMetadata()
}
