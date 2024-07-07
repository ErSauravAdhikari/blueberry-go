package blueberry

import "time"

// TaskExecution represents the execution details of a task
type TaskExecution struct {
	ID        int                    `json:"id"`
	TaskName  string                 `json:"task_name"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  string                 `json:"duration"`
	Params    map[string]interface{} `json:"params"`
	Status    string                 `json:"status"`
}

// TaskInfo represents the task and its schedules
type TaskInfo struct {
	TaskName  string         `json:"task_name"`
	Schedules []ScheduleInfo `json:"schedules"`
}

type getTaskRunLogResponse struct {
	Logs []TaskRunLog `json:"logs"`
}

type getTaskExecutionsResponse struct {
	TaskExecutions []TaskExecution `json:"task_executions"`
}

type ExecuteTaskRequest struct {
	Params TaskParams `json:"params"`
}

type ErrorResponse struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type GenericResponse map[string]interface{}
