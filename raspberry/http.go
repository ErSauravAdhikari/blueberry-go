package rasberry

import (
	_ "github.com/ersauravadhikari/raspberry-go/docs"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// @title Raspberry API
// @version 1.0
// @description This is a simple task scheduler API.

// @BasePath /api/

// RunAPI starts the API server
// @Summary Start API server
// @Description Start the API server to manage tasks and schedules
// @Produce json
// @Success 200 {object} string "API server started"
// @Router / [get]
func (r *Raspberry) RunAPI(port string) {
	e := echo.New()

	e.GET("/api/tasks", r.getTasks)
	e.GET("/api/task/:name/executions", r.getTaskExecutions)
	e.GET("/api/task_run/:id/logs", r.getTaskRunLogs)

	// Swagger docs endpoint
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	e.Logger.Fatal(e.Start(":" + port))
}
