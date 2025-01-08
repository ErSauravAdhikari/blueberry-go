package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	blueberry "github.com/ersauravadhikari/blueberry-go/blueberry"
	"github.com/ersauravadhikari/blueberry-go/blueberry/store"
	"github.com/labstack/echo/v4"
	"github.com/robfig/cron/v3"
)

// Schedule request/response structures
type ScheduleRequest struct {
	TaskName string               `json:"task_name"`
	Params   blueberry.TaskParams `json:"params"`
	Schedule string               `json:"schedule"`
}

type ScheduleResponse struct {
	EntryID int    `json:"entry_id"`
	Message string `json:"message"`
}

type UpdateScheduleRequest struct {
	Params   blueberry.TaskParams `json:"params"`
	Schedule string               `json:"schedule,omitempty"`
}

// Simple task that logs to console
func simpleTask(ctx context.Context, params blueberry.TaskParams, logger *blueberry.Logger) error {
	msg := fmt.Sprintf("Executing simple task with params: %v at %v", params, time.Now())
	return logger.Info(msg)
}

// Custom route handlers
type ScheduleHandler struct {
	bb    *blueberry.BlueBerry
	tasks map[string]*blueberry.Task
}

func NewScheduleHandler(bb *blueberry.BlueBerry) *ScheduleHandler {
	return &ScheduleHandler{
		bb:    bb,
		tasks: make(map[string]*blueberry.Task),
	}
}

func (h *ScheduleHandler) RegisterTask(name string, task *blueberry.Task) {
	h.tasks[name] = task
}

// Handler to create new schedule
func (h *ScheduleHandler) CreateSchedule(c echo.Context) error {
	req := new(ScheduleRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	task, exists := h.tasks[req.TaskName]
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	schedule, err := task.RegisterSchedule(req.Params, req.Schedule)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, ScheduleResponse{
		EntryID: int(schedule.EntryID),
		Message: "Schedule created successfully",
	})
}

// Handler to update schedule
func (h *ScheduleHandler) UpdateSchedule(c echo.Context) error {
	taskName := c.Param("taskName")
	entryIDStr := c.Param("entryID")

	entryID, err := strconv.Atoi(entryIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid entry ID"})
	}

	task, exists := h.tasks[taskName]
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	req := new(UpdateScheduleRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Delete existing schedule
	task.DeleteSchedule(cron.EntryID(entryID))

	// Create new schedule
	schedule, err := task.RegisterSchedule(req.Params, req.Schedule)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, ScheduleResponse{
		EntryID: int(schedule.EntryID),
		Message: "Schedule updated successfully",
	})
}

// Handler to delete schedule
func (h *ScheduleHandler) DeleteSchedule(c echo.Context) error {
	taskName := c.Param("taskName")
	entryIDStr := c.Param("entryID")

	entryID, err := strconv.Atoi(entryIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid entry ID"})
	}

	task, exists := h.tasks[taskName]
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
	}

	task.DeleteSchedule(cron.EntryID(entryID))
	return c.JSON(http.StatusOK, map[string]string{"message": "Schedule deleted successfully"})
}

func main() {
	db, err := store.NewSQLiteDB("task_scheduler.db")
	if err != nil {
		log.Fatalf("Failed to initialize SQLite: %v", err)
	}
	defer db.Close()

	bb := blueberry.NewBlueBerryInstance(db)

	// Define task schema
	taskSchema := blueberry.NewTaskSchema(blueberry.TaskParamDefinition{
		"message": blueberry.TypeString,
		"count":   blueberry.TypeInt,
	})

	// Register task
	simpleTask1, err := bb.RegisterTask("simple_task", simpleTask, taskSchema)
	if err != nil {
		log.Fatalf("Failed to register task: %v", err)
	}

	// Create schedule handler
	scheduleHandler := NewScheduleHandler(bb)
	scheduleHandler.RegisterTask("simple_task", simpleTask1)

	// Get Echo instance with custom configuration
	e, err := bb.GetEcho(&blueberry.Config{
		WebUIPath: "/admin",
		APIPath:   "/api/v1",
	})
	if err != nil {
		log.Fatalf("Failed to get Echo instance: %v", err)
	}

	// Register custom routes
	customAPI := e.Group("/api/v1/schedules")
	customAPI.POST("", scheduleHandler.CreateSchedule)
	customAPI.PUT("/:taskName/:entryID", scheduleHandler.UpdateSchedule)
	customAPI.DELETE("/:taskName/:entryID", scheduleHandler.DeleteSchedule)

	// Handle system signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v. Shutting down...", sig)
		bb.Shutdown()
		db.Close()
		os.Exit(0)
	}()

	bb.InitTaskScheduler()
	e.Start(":8080")
}
