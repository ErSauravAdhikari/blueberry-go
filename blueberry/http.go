package blueberry

import (
	_ "github.com/ersauravadhikari/blueberry-go/docs"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
)

type Config struct {
	WebUIPath string // base path for web UI routes (e.g., "/bb_admin")
	APIPath   string // base path for API routes (e.g., "/bb_api")
}

// setupCore initializes the Echo instance with common middleware
func (r *BlueBerry) setupCore() (*echo.Echo, error) {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Load templates
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	e.Renderer = &Template{templates: templates}
	return e, nil
}

// GetEcho returns a configured Echo instance with routes mounted at specified paths
func (r *BlueBerry) GetEcho(cfg *Config) (*echo.Echo, error) {
	e, err := r.setupCore()
	if err != nil {
		return nil, err
	}

	// Default paths if not specified
	webPath := "/"
	apiPath := "/api"

	if cfg != nil {
		if cfg.WebUIPath != "" {
			webPath = cfg.WebUIPath
		}
		if cfg.APIPath != "" {
			apiPath = cfg.APIPath
		}
	}

	// Setup Web UI routes
	webGroup := e.Group(webPath)
	r.setupWebRoutes(webGroup)

	// Setup API routes
	apiGroup := e.Group(apiPath)
	r.setupAPIRoutes(apiGroup)

	// Swagger docs endpoint
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	return e, nil
}

// setupWebRoutes configures all web UI routes
func (r *BlueBerry) setupWebRoutes(web *echo.Group) {
	web.GET("/login", r.serveLoginPage)
	web.POST("/login", r.handleLogin)

	if len(r.webOnlyPasswords) > 0 {
		web.Use(r.webAuthMiddleware)
	}

	web.GET("", r.listTasks) // Note: changed from "/" to "" since we're in a group
	web.GET("/task/:name", r.showTask)
	web.GET("/task/:name/run", r.executeTaskForm)
	web.POST("/task/:name/execute", r.handleExecuteTask)
	web.GET("/execution/:id", r.showExecution)
	web.POST("/execution/:id/cancel", r.cancelExecutionByIDWeb)
	web.GET("/execution/:id/download", r.downloadLogs)
}

// setupAPIRoutes configures all API routes
func (r *BlueBerry) setupAPIRoutes(api *echo.Group) {
	if len(r.apiKeys) > 0 {
		api.Use(r.apiKeyAuthMiddleware)
	}

	api.GET("/tasks", r.getTasks)
	api.GET("/task/:name/executions", r.getTaskExecutions)
	api.GET("/task_run/:id/logs", r.getTaskRunLogs)
	api.POST("/execution/:id/cancel", r.cancelExecutionByID)
	api.POST("/task/:name/execute", r.executeTaskByName)
}

// @title BlueBerry API
// @version 1.0
// @description This is a simple task scheduler API.
// @BasePath /api/
// @securityDefinitions.apiKey ApiKeyAuth
// @in query
// @name api_key
// RunAPI starts the API server
// @Summary Start API server
// @Description Start the API server to manage tasks and schedules
// @Produce json
// @Success 200 {object} string "API server started"
// @Router / [get]
func (r *BlueBerry) RunAPI(port string) error {
	e, err := r.GetEcho(nil) // Use default paths
	if err != nil {
		return err
	}
	return e.Start(":" + port)
}
