package rasberry

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

// APIKeyAuthMiddleware checks the API key for API authentication
func (r *Raspberry) APIKeyAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		apiKey := c.QueryParam("api_key")
		r.apiKeysMux.RLock()
		defer r.apiKeysMux.RUnlock()
		if _, ok := r.apiKeys[apiKey]; ok {
			return next(c)
		}
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid API key")
	}
}

// getTasks returns all registered tasks and their schedules
// @Summary Get all registered tasks and their schedules
// @Description Get details of all registered tasks and their schedules
// @Tags Task
// @Produce json
// @Success 200 {array} TaskInfo
// @Router /tasks [get]
func (r *Raspberry) getTasks(c echo.Context) error {
	var tasks []TaskInfo

	r.tasks.Range(func(key, value interface{}) bool {
		taskName := key.(string)
		schedules := r.getSchedules(taskName)

		tasks = append(tasks, TaskInfo{
			TaskName:  taskName,
			Schedules: schedules,
		})
		return true
	})

	return c.JSON(http.StatusOK, tasks)
}

// getTaskExecutions returns all executions for a specific task
// @Summary Get all executions for a specific task
// @Description Get all executions for a specific task by name
// @Param name path string true "Task Name"
// @Tags Executions
// @Produce json
// @Success 200 {array} getTaskExecutionsResponse
// @Router /task/{name}/executions [get]
func (r *Raspberry) getTaskExecutions(c echo.Context) error {
	taskName := c.Param("name")
	taskRuns, err := r.db.GetTaskRuns(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	var taskExecutions []TaskExecution
	for _, taskRun := range taskRuns {
		if taskRun.TaskName == taskName {
			var duration string
			var status string
			if taskRun.EndTime.IsZero() {
				duration = "ongoing"
				status = "ongoing"
			} else {
				duration = taskRun.EndTime.Sub(taskRun.StartTime).String()
				status = taskRun.Status
			}

			taskExecutions = append(taskExecutions, TaskExecution{
				ID:        taskRun.ID,
				TaskName:  taskRun.TaskName,
				StartTime: taskRun.StartTime,
				EndTime:   taskRun.EndTime,
				Duration:  duration,
				Params:    taskRun.Params,
				Status:    status,
			})
		}
	}

	return c.JSON(http.StatusOK, getTaskExecutionsResponse{
		TaskExecutions: taskExecutions,
	})
}

// getTaskRunLogs returns all logs for a specific task run
// @Summary Get all logs for a specific task run
// @Description Get all logs for a specific task run by ID with pagination and log level filtering
// @Param id path int true "Task Run ID"
// @Param level query string false "Log level filter" Enums(info, debug, error, success, all) default(info)
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Tags Logs
// @Produce json
// @Success 200 {array} getTaskRunLogResponse
// @Router /task_run/{id}/logs [get]
func (r *Raspberry) getTaskRunLogs(c echo.Context) error {
	runID := c.Param("id")
	taskRunID, err := strconv.Atoi(runID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid task run ID"})
	}

	level := c.QueryParam("level")
	if level == "" {
		level = "info"
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(c.QueryParam("size"))
	if err != nil || size < 1 {
		size = 10
	}

	logs, err := r.db.GetPaginatedTaskRunLogs(context.Background(), taskRunID, level, page, size)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, getTaskRunLogResponse{
		Logs: logs,
	})
}

// CancelExecutionByID cancels a specific task execution by ID
// @Summary Cancel a specific task execution by ID
// @Description Cancel a specific task execution by its ID
// @Param id path int true "Task Execution ID"
// @Tags Executions
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 404 {object} object
// @Router /execution/{id}/cancel [post]
func (r *Raspberry) cancelExecutionByID(c echo.Context) error {
	executionID := c.Param("id")
	taskRunID, err := strconv.Atoi(executionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid execution ID"})
	}

	err = r.CancelExecutionByID(taskRunID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Execution cancelled successfully"})
}

// ExecuteTaskByName handles the execution of a task by its name
// @Summary Execute a task by name
// @Description Execute a specified task by its name with the provided parameters
// @Accept json
// @Produce json
// @Param name path string true "Task Name"
// @Param params body ExecuteTaskRequest true "Task Parameters"
// @Success 200 {object} GenericResponse "Task executed successfully"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Failure 404 {object} ErrorResponse "Task not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /task/{name}/execute [post]
// @Security ApiKeyAuth
func (r *Raspberry) ExecuteTaskByName(c echo.Context) error {
	taskName := c.Param("name")
	var req ExecuteTaskRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			"validation",
			err.Error(),
		})
	}

	taskInterface, ok := r.tasks.Load(taskName)
	if !ok {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			"user",
			"Invalid task name",
		})
	}

	task := taskInterface.(*Task)
	if err := task.ValidateParams(req.Params); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			"validation",
			err.Error(),
		})
	}

	if err := task.ExecuteNow(req.Params); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			"system",
			err.Error(),
		})
	}

	return c.JSON(http.StatusOK, GenericResponse{
		"Task scheduled to run @now successfully",
	})
}
