I'll create a comprehensive tutorial explaining how to extend BlueBerry with custom schedule management endpoints.

# Tutorial: Extending BlueBerry with Custom Schedule Management

## Introduction
This tutorial demonstrates how to extend BlueBerry's functionality by adding custom endpoints to manage task schedules. We'll build a complete example that allows you to create, update, and delete schedules via REST API endpoints.

## Prerequisites
- Basic understanding of Go programming
- Go installed on your system
- Basic understanding of REST APIs
- Familiarity with JSON

## Step 1: Setting up the Project Structure

First, let's define our data structures:

```go
// Schedule request/response structures
type ScheduleRequest struct {
    TaskName string                 `json:"task_name"`
    Params   blueberry.TaskParams   `json:"params"`
    Schedule string                 `json:"schedule"`
}

type ScheduleResponse struct {
    EntryID int    `json:"entry_id"`
    Message string `json:"message"`
}

type UpdateScheduleRequest struct {
    Params   blueberry.TaskParams   `json:"params"`
    Schedule string                 `json:"schedule,omitempty"`
}
```

Let's understand these structures:
- `ScheduleRequest`: Used when creating a new schedule
  - `TaskName`: Name of the task to schedule
  - `Params`: Parameters required by the task
  - `Schedule`: Cron expression or interval (e.g., "@every 1m")

- `ScheduleResponse`: Response after schedule operations
  - `EntryID`: Unique identifier for the schedule
  - `Message`: Operation status message

- `UpdateScheduleRequest`: Used when updating an existing schedule
  - `Params`: New parameters for the task
  - `Schedule`: Optional new schedule timing

## Step 2: Creating a Simple Task

Let's create a basic task that logs messages:

```go
func simpleTask(ctx context.Context, params blueberry.TaskParams, logger *blueberry.Logger) error {
    msg := fmt.Sprintf("Executing simple task with params: %v at %v", params, time.Now())
    return logger.Info(msg)
}
```

This task:
1. Accepts context, parameters, and a logger
2. Formats a message with the current parameters and timestamp
3. Logs the message using BlueBerry's logger
4. Returns any error that occurred during logging

## Step 3: Creating the Schedule Handler

```go
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
```

The `ScheduleHandler`:
- Stores a reference to the BlueBerry instance
- Maintains a map of registered tasks
- Provides a method to register tasks for scheduling

## Step 4: Implementing Schedule Management Endpoints

### Create Schedule Handler
```go
func (h *ScheduleHandler) CreateSchedule(c echo.Context) error {
    // Parse request
    req := new(ScheduleRequest)
    if err := c.Bind(req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    // Find task
    task, exists := h.tasks[req.TaskName]
    if !exists {
        return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
    }

    // Create schedule
    schedule, err := task.RegisterSchedule(req.Params, req.Schedule)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    return c.JSON(http.StatusCreated, ScheduleResponse{
        EntryID: int(schedule.EntryID),
        Message: "Schedule created successfully",
    })
}
```

This handler:
1. Parses the incoming JSON request
2. Validates that the requested task exists
3. Creates a new schedule for the task
4. Returns the schedule ID and success message

### Update Schedule Handler
```go
func (h *ScheduleHandler) UpdateSchedule(c echo.Context) error {
    // Get path parameters
    taskName := c.Param("taskName")
    entryIDStr := c.Param("entryID")
    
    // Convert entryID to integer
    entryID, err := strconv.Atoi(entryIDStr)
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid entry ID"})
    }

    // Find task
    task, exists := h.tasks[taskName]
    if !exists {
        return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
    }

    // Parse request
    req := new(UpdateScheduleRequest)
    if err := c.Bind(req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    // Update schedule (delete and recreate)
    task.DeleteSchedule(cron.EntryID(entryID))
    schedule, err := task.RegisterSchedule(req.Params, req.Schedule)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    return c.JSON(http.StatusOK, ScheduleResponse{
        EntryID: int(schedule.EntryID),
        Message: "Schedule updated successfully",
    })
}
```

This handler:
1. Extracts task name and schedule ID from the URL
2. Validates the task exists
3. Parses the update request
4. Removes the old schedule
5. Creates a new schedule with updated parameters
6. Returns the new schedule ID

### Delete Schedule Handler
```go
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
```

This handler:
1. Extracts task name and schedule ID from the URL
2. Validates the task exists
3. Deletes the schedule
4. Returns success message

## Step 5: Putting It All Together

```go
func main() {
    // Initialize database
    db, err := store.NewSQLiteDB("task_scheduler.db")
    if err != nil {
        log.Fatalf("Failed to initialize SQLite: %v", err)
    }
    defer db.Close()

    // Create BlueBerry instance
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

    // Create and setup schedule handler
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

    // Setup shutdown handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        sig := <-sigChan
        log.Printf("Received signal: %v. Shutting down...", sig)
        bb.Shutdown()
        db.Close()
        os.Exit(0)
    }()

    // Initialize and start
    bb.InitTaskScheduler()
    e.Start(":8080")
}
```

## Step 6: Testing the API

You can test the endpoints using curl:

```bash
# Create a new schedule
curl -X POST http://localhost:8080/api/v1/schedules \
  -H "Content-Type: application/json" \
  -d '{
    "task_name": "simple_task",
    "params": {
      "message": "Hello, World!",
      "count": 1
    },
    "schedule": "@every 1m"
  }'

# Update a schedule
curl -X PUT http://localhost:8080/api/v1/schedules/simple_task/1 \
  -H "Content-Type: application/json" \
  -d '{
    "params": {
      "message": "Updated message",
      "count": 2
    },
    "schedule": "@every 2m"
  }'

# Delete a schedule
curl -X DELETE http://localhost:8080/api/v1/schedules/simple_task/1
```

## Conclusion
This tutorial demonstrated how to:
1. Create custom endpoints for schedule management
2. Integrate with BlueBerry's task and scheduling system
3. Handle schedule creation, updates, and deletion
4. Properly structure the code for maintainability
5. Implement error handling and validation

For next steps do refer to [Schedule ID Persistance](./Schedule%20ID%20Persistance.md)