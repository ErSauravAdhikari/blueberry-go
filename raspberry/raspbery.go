package rasberry

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type TaskParamType string

const (
	TypeInt    TaskParamType = "int"
	TypeBool   TaskParamType = "bool"
	TypeString TaskParamType = "string"
	TypeSelect TaskParamType = "select"
	TypeEnum   TaskParamType = "enum"
	TypeFloat  TaskParamType = "float"
)

type TaskParamDefinition map[string]TaskParamType
type TaskParams map[string]interface{}

// TaskSchema is used to define the schema for the task
type TaskSchema struct {
	Fields TaskParamDefinition // map[fieldName]fieldType
}

// NewTaskSchema is a helper function to create a new TaskSchema
func NewTaskSchema(fields TaskParamDefinition) TaskSchema {
	return TaskSchema{
		Fields: fields,
	}
}

type ScheduleInfo struct {
	Schedule      string                 `json:"schedule"`
	Params        map[string]interface{} `json:"params"`
	NextExecution int64                  `json:"next_execution_ts"`
	EntryID       cron.EntryID           `json:"-"`
}

type Task struct {
	name      string
	taskFunc  func(context.Context, TaskParams, *Logger) error
	raspberry *Raspberry
	schema    TaskSchema
}

type Raspberry struct {
	db        DB
	cron      *cron.Cron
	tasks     sync.Map
	taskMux   sync.Mutex
	schedules sync.Map // To store schedules per task
	executing sync.Map // To track currently executing tasks

	apiKeys          map[string]string
	apiKeysMux       sync.RWMutex
	usersMux         sync.RWMutex
	webOnlyPasswords map[string]string
}

func NewRaspberryInstance(db DB) *Raspberry {
	return &Raspberry{
		db:               db,
		cron:             cron.New(),
		apiKeys:          make(map[string]string),
		webOnlyPasswords: make(map[string]string),
	}
}

func (r *Raspberry) AddWebOnlyPasswordAuth(username, password string) {
	r.usersMux.Lock()
	defer r.usersMux.Unlock()
	r.webOnlyPasswords[username] = password
}

func (r *Raspberry) AddAPIOnlyKeyAuth(apiKey, description string) {
	r.apiKeysMux.Lock()
	defer r.apiKeysMux.Unlock()
	r.apiKeys[apiKey] = description
}

func (r *Raspberry) RegisterTask(taskName string, taskFunc func(context.Context, TaskParams, *Logger) error, schema TaskSchema) (*Task, error) {
	if err := validateSchema(schema); err != nil {
		return nil, err
	}

	r.taskMux.Lock()
	defer r.taskMux.Unlock()
	task := &Task{
		name:      taskName,
		taskFunc:  taskFunc,
		schema:    schema,
		raspberry: r,
	}
	r.tasks.Store(taskName, task)
	return task, nil
}

func validateSchema(schema TaskSchema) error {
	supportedTypes := map[TaskParamType]struct{}{
		TypeInt:    {},
		TypeBool:   {},
		TypeString: {},
		TypeSelect: {},
		TypeEnum:   {},
		TypeFloat:  {},
	}
	for _, fieldType := range schema.Fields {
		if _, ok := supportedTypes[fieldType]; !ok {
			return fmt.Errorf("unsupported field type: %s", fieldType)
		}
	}
	return nil
}

func (t *Task) ValidateParams(params TaskParams) error {
	// Check if all required parameters are present
	for key, expectedType := range t.schema.Fields {
		value, ok := params[key]
		if !ok {
			return fmt.Errorf("missing required parameter: %s", key)
		}

		if !validateType(value, expectedType) {
			return fmt.Errorf("parameter %s should be of type %s", key, expectedType)
		}
	}

	// Check if there are any unexpected parameters
	for key := range params {
		if _, ok := t.schema.Fields[key]; !ok {
			return fmt.Errorf("unexpected parameter: %s", key)
		}
	}

	return nil
}

func validateType(value interface{}, expectedType TaskParamType) bool {
	v := reflect.ValueOf(value)
	switch expectedType {
	case TypeInt:
		return v.Kind() == reflect.Int
	case TypeBool:
		return v.Kind() == reflect.Bool
	case TypeString:
		return v.Kind() == reflect.String
	case TypeFloat:
		return v.Kind() == reflect.Float64 || v.Kind() == reflect.Float32
	// Add additional case handling for "select" and "enum" as needed
	default:
		return false
	}
}

func (t *Task) RegisterSchedule(params TaskParams, schedule string) error {
	if err := t.ValidateParams(params); err != nil {
		return err
	}

	entryID, err := t.raspberry.cron.AddFunc(schedule, func() {
		t.ExecuteNow(context.Background(), params)
	})
	if err != nil {
		return err
	}

	// Store the schedule with the entry ID
	scheduleInfo := ScheduleInfo{
		Schedule:      schedule,
		Params:        params,
		NextExecution: t.raspberry.cron.Entry(entryID).Next.UTC().Unix(),
		EntryID:       entryID,
	}
	t.raspberry.storeSchedule(t.name, scheduleInfo)

	return nil
}

func (t *Task) ExecuteNow(ctx context.Context, params TaskParams) error {
	if err := t.ValidateParams(params); err != nil {
		return err
	}

	go func(ctx context.Context, params TaskParams) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		taskRun := &TaskRun{
			TaskName:  t.name,
			StartTime: time.Now().UTC(),
			Params:    params,
			Status:    "started",
		}

		err := t.raspberry.db.SaveTaskRun(ctx, taskRun)
		if err != nil {
			fmt.Printf("unable to log task start: %v\n", err)
			return
		}

		t.raspberry.executing.Store(taskRun.ID, cancel)
		defer t.raspberry.executing.Delete(taskRun.ID)

		logger := &Logger{taskRun: taskRun, db: t.raspberry.db}
		err = t.taskFunc(ctx, params, logger)
		if err != nil {
			taskRun.Status = "failed"
			_ = logger.Error("Task failed due to: " + err.Error())
		} else {
			taskRun.Status = "completed"
		}
		taskRun.EndTime = time.Now().UTC()

		err = t.raspberry.db.SaveTaskRun(ctx, taskRun)
		if err != nil {
			_ = logger.Error("Unable to save task run due to: " + err.Error())
		}
	}(ctx, params)

	return nil
}

func (r *Raspberry) storeSchedule(taskName string, scheduleInfo ScheduleInfo) {
	schedules, _ := r.schedules.LoadOrStore(taskName, []ScheduleInfo{})
	schedules = append(schedules.([]ScheduleInfo), scheduleInfo)
	r.schedules.Store(taskName, schedules)
}

func (r *Raspberry) getSchedules(taskName string) []ScheduleInfo {
	loadedSchedules, ok := r.schedules.Load(taskName)
	if !ok {
		return nil
	}

	schedules := loadedSchedules.([]ScheduleInfo)

	for i := range schedules {
		// Retrieve the next execution time using the entry ID
		entry := r.cron.Entry(schedules[i].EntryID)
		schedules[i].NextExecution = entry.Next.UTC().Unix()
	}

	return schedules
}

func (r *Raspberry) InitTaskScheduler() {
	r.cron.Start()
}

func (r *Raspberry) Shutdown() {
	// Cancel all running tasks
	r.executing.Range(func(key, value interface{}) bool {
		executionID := key.(int)
		cancel := value.(context.CancelFunc)
		cancel()

		// Log the cancellation to the database
		taskRun, err := r.db.GetTaskRunByID(context.Background(), executionID)
		if err == nil {
			taskRun.Status = "cancelled"
			taskRun.EndTime = time.Now().UTC()
			_ = r.db.SaveTaskRun(context.Background(), taskRun)
		}

		return true
	})
}

func (r *Raspberry) CancelExecutionByID(executionID int) error {
	cancel, ok := r.executing.Load(executionID)
	if !ok {
		return fmt.Errorf("execution ID %d not found or already completed", executionID)
	}

	// Remove the cancel function from the map
	defer r.executing.Delete(executionID)
	cancel.(context.CancelFunc)()

	// Log the cancellation to the database
	taskRun, err := r.db.GetTaskRunByID(context.Background(), executionID)
	if err != nil {
		return fmt.Errorf("failed to retrieve task run: %v", err)
	}

	taskRun.Status = "cancelled"
	taskRun.EndTime = time.Now().UTC()
	if err := r.db.SaveTaskRun(context.Background(), taskRun); err != nil {
		return fmt.Errorf("failed to save task run: %v", err)
	}

	return nil
}
