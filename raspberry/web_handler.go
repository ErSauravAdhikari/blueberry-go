package rasberry

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// Middleware to check cookie for web authentication
func (r *Raspberry) WebAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
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
	schedules := r.getSchedules(taskName)
	if schedules == nil {
		schedules = []ScheduleInfo{}
	}

	executions, err := r.db.GetTaskRuns(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	var taskExecutions []TaskRun
	for _, execution := range executions {
		if execution.TaskName == taskName {
			taskExecutions = append(taskExecutions, execution)
		}
	}

	data := struct {
		TaskName   string
		Schedules  []ScheduleInfo
		Executions []TaskRun
	}{
		TaskName:   taskName,
		Schedules:  schedules,
		Executions: taskExecutions,
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
