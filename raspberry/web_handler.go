package rasberry

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// TemplateScheduleInfo is used for rendering schedules in the template
type TemplateScheduleInfo struct {
	Schedule               string
	FormattedNextExecution string
}

// TemplateTaskRun is used for rendering task runs in the template
type TemplateTaskRun struct {
	ID                 int
	FormattedStartTime string
	FormattedEndTime   string
	Status             string
}

const tasksPerPage = 20

// formatTime formats a given time.Time to a readable string
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// formatUnixTimestamp formats a given Unix timestamp to a readable string
func formatUnixTimestamp(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}

// Middleware to check cookie for web authentication
func (r *Raspberry) webAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, err := c.Cookie("auth")
		if err != nil || cookie.Value != "authenticated" {
			return c.Redirect(http.StatusFound, "/login")
		}
		return next(c)
	}
}

// Serve the login page
func (r *Raspberry) serveLoginPage(c echo.Context) error {
	return c.Render(http.StatusOK, "login.goml", nil)
}

// Handle login form submission
func (r *Raspberry) handleLogin(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	r.usersMux.RLock()
	defer r.usersMux.RUnlock()
	if pass, ok := r.webOnlyPasswords[username]; ok && pass == password {
		cookie := new(http.Cookie)
		cookie.Name = "auth"
		cookie.Value = "authenticated"
		cookie.Expires = time.Now().Add(24 * time.Hour)
		c.SetCookie(cookie)
		return c.Redirect(http.StatusFound, "/")
	}
	return c.Redirect(http.StatusFound, "/login")
}

// listTasks renders the index page with all tasks
func (r *Raspberry) listTasks(c echo.Context) error {
	var tasks []TaskInfo

	r.tasks.Range(func(key, value interface{}) bool {
		taskName := key.(string)
		schedules := r.getSchedules(taskName)
		if schedules == nil {
			schedules = []ScheduleInfo{}
		}

		tasks = append(tasks, TaskInfo{
			TaskName:  taskName,
			Schedules: schedules,
		})
		return true
	})

	return c.Render(http.StatusOK, "index.goml", tasks)
}

// showTask renders the task page with its schedules and past executions
func (r *Raspberry) showTask(c echo.Context) error {
	taskName := c.Param("name")
	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || page < 1 {
		page = 1
	}

	schedules := r.getSchedules(taskName)
	if schedules == nil {
		schedules = []ScheduleInfo{}
	}

	paginatedTasks, err := r.db.GetPaginatedTaskRunsForTaskName(context.Background(), taskName, page, tasksPerPage)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	totalTasks, err := r.db.GetTaskRunsCountForTaskName(context.Background(), taskName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	totalPages := (totalTasks + tasksPerPage - 1) / tasksPerPage

	var templateSchedules []TemplateScheduleInfo
	for _, schedule := range schedules {
		templateSchedules = append(templateSchedules, TemplateScheduleInfo{
			Schedule:               schedule.Schedule,
			FormattedNextExecution: formatUnixTimestamp(schedule.NextExecution),
		})
	}

	var templateTaskRuns []TemplateTaskRun
	for _, execution := range paginatedTasks {
		templateTaskRuns = append(templateTaskRuns, TemplateTaskRun{
			ID:                 execution.ID,
			FormattedStartTime: formatTime(execution.StartTime),
			FormattedEndTime:   formatTime(execution.EndTime),
			Status:             execution.Status,
		})
	}

	data := struct {
		TaskName   string
		Schedules  []TemplateScheduleInfo
		Executions []TemplateTaskRun
		Page       int
		TotalPages int
	}{
		TaskName:   taskName,
		Schedules:  templateSchedules,
		Executions: templateTaskRuns,
		Page:       page,
		TotalPages: totalPages,
	}

	return c.Render(http.StatusOK, "task.goml", data)
}

// showExecution renders the execution page with its logs
func (r *Raspberry) showExecution(c echo.Context) error {
	executionID := c.Param("id")
	taskRunID, err := strconv.Atoi(executionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid execution ID"})
	}

	executions, err := r.db.GetTaskRuns(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	var execution TaskRun
	for _, exec := range executions {
		if exec.ID == taskRunID {
			execution = exec
			break
		}
	}

	logs, err := r.db.GetTaskRunLogs(context.Background(), taskRunID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	data := struct {
		TaskRun
		Logs []TaskRunLog
	}{
		TaskRun: execution,
		Logs:    logs,
	}

	return c.Render(http.StatusOK, "execution.goml", data)
}

func (r *Raspberry) cancelExecutionByIDWeb(c echo.Context) error {
	executionID := c.Param("id")
	taskRunID, err := strconv.Atoi(executionID)
	if err != nil {
		return c.Render(http.StatusBadRequest, "error.html", map[string]string{"error": "Invalid execution ID"})
	}

	err = r.CancelExecutionByID(taskRunID)
	if err != nil {
		return c.Render(http.StatusNotFound, "error.html", map[string]string{"error": err.Error()})
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/execution/%d", taskRunID))
}

// executeTaskForm renders the form for executing a task
func (r *Raspberry) executeTaskForm(c echo.Context) error {
	taskName := c.Param("name")
	taskInterface, ok := r.tasks.Load(taskName)
	if !ok {
		return c.JSON(http.StatusNotFound, "Task not found")
	}

	task := taskInterface.(*Task)

	data := struct {
		TaskName string
		Schema   TaskSchema
	}{
		TaskName: task.name,
		Schema:   task.schema,
	}

	return c.Render(http.StatusOK, "task_run.goml", data)
}

// handleExecuteTask processes the form submission to execute a task
func (r *Raspberry) handleExecuteTask(c echo.Context) error {
	taskName := c.Param("name")
	taskInterface, ok := r.tasks.Load(taskName)
	if !ok {
		return c.JSON(http.StatusNotFound, "Task not found")
	}

	task := taskInterface.(*Task)

	params := make(TaskParams)
	for key := range task.schema.Fields {
		value := c.FormValue(key)
		if task.schema.Fields[key] == TypeInt {
			intVal, err := strconv.Atoi(value)
			if err != nil {
				return c.JSON(http.StatusBadRequest, fmt.Sprintf("Invalid value for %s", key))
			}
			params[key] = intVal
		} else if task.schema.Fields[key] == TypeFloat {
			floatVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, fmt.Sprintf("Invalid value for %s", key))
			}
			params[key] = floatVal
		} else if task.schema.Fields[key] == TypeBool {
			boolVal := value == "on"
			params[key] = boolVal
			fmt.Printf("Parameter %s: %v\n", key, boolVal)
		} else {
			params[key] = value
		}
	}

	taskID, err := task.ExecuteNow(params)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}

	return c.Redirect(http.StatusFound, fmt.Sprintf("/execution/%d", taskID))
}
