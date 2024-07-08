package blueberry

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type TaskParamType string

const (
	TypeInt    TaskParamType = "int"
	TypeBool   TaskParamType = "bool"
	TypeString TaskParamType = "string"
	TypeFloat  TaskParamType = "float"
)

type TaskParamDefinition map[string]TaskParamType

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
	blueBerry *BlueBerry
	schema    TaskSchema
}

type BlueBerry struct {
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

func NewBlueBerryInstance(db DB) *BlueBerry {
	return &BlueBerry{
		db:               db,
		cron:             cron.New(),
		apiKeys:          make(map[string]string),
		webOnlyPasswords: make(map[string]string),
	}
}

func (r *BlueBerry) AddWebOnlyPasswordAuth(username, password string) {
	r.usersMux.Lock()
	defer r.usersMux.Unlock()
	r.webOnlyPasswords[username] = password
}

func (r *BlueBerry) AddAPIOnlyKeyAuth(apiKey, description string) {
	r.apiKeysMux.Lock()
	defer r.apiKeysMux.Unlock()
	r.apiKeys[apiKey] = description
}

func (r *BlueBerry) RegisterTask(taskName string, taskFunc TaskFunc, schema TaskSchema) (*Task, error) {
	if err := validateSchema(schema); err != nil {
		return nil, err
	}

	r.taskMux.Lock()
	defer r.taskMux.Unlock()
	task := &Task{
		name:      taskName,
		taskFunc:  taskFunc,
		schema:    schema,
		blueBerry: r,
	}
	r.tasks.Store(taskName, task)
	return task, nil
}

func validateSchema(schema TaskSchema) error {
	supportedTypes := map[TaskParamType]struct{}{
		TypeInt:    {},
		TypeBool:   {},
		TypeString: {},
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
	// Check if all required parameters are present and validate their types
	for key, expectedType := range t.schema.Fields {
		_, ok := params[key]
		if !ok {
			return fmt.Errorf("missing required parameter: %s", key)
		}

		if err := validateType(params, key, expectedType); err != nil {
			return err
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

func validateType(params TaskParams, key string, expectedType TaskParamType) error {
	value := params[key]
	v := reflect.ValueOf(value)

	switch expectedType {
	case TypeInt:
		if v.Kind() == reflect.Int {
			return nil
		}
		if v.Kind() == reflect.Float64 {
			params[key] = int(value.(float64))
			return nil
		}
		if v.Kind() == reflect.String {
			intVal, err := strconv.Atoi(value.(string))
			if err != nil {
				return fmt.Errorf("parameter %s should be convertible to int", key)
			}
			params[key] = intVal
			return nil
		}
		return fmt.Errorf("parameter %s should be of type int", key)

	case TypeBool:
		if v.Kind() == reflect.Bool {
			return nil
		}
		return fmt.Errorf("parameter %s should be of type bool", key)

	case TypeString:
		if v.Kind() == reflect.String {
			return nil
		}
		return fmt.Errorf("parameter %s should be of type string", key)

	case TypeFloat:
		if v.Kind() == reflect.Float64 || v.Kind() == reflect.Float32 {
			return nil
		}
		if v.Kind() == reflect.Int {
			params[key] = float64(value.(int))
			return nil
		}
		if v.Kind() == reflect.String {
			floatVal, err := strconv.ParseFloat(value.(string), 64)
			if err != nil {
				return fmt.Errorf("parameter %s should be convertible to float", key)
			}
			params[key] = floatVal
			return nil
		}
		return fmt.Errorf("parameter %s should be of type float", key)

	default:
		return fmt.Errorf("unsupported parameter type %s", expectedType)
	}
}

func (t *Task) RegisterSchedule(params TaskParams, schedule string) error {
	if err := t.ValidateParams(params); err != nil {
		return err
	}

	entryID, err := t.blueBerry.cron.AddFunc(schedule, func() {
		t.ExecuteNow(params)
	})
	if err != nil {
		return err
	}

	// Store the schedule with the entry ID
	scheduleInfo := ScheduleInfo{
		Schedule:      schedule,
		Params:        params,
		NextExecution: t.blueBerry.cron.Entry(entryID).Next.UTC().Unix(),
		EntryID:       entryID,
	}
	t.blueBerry.storeSchedule(t.name, scheduleInfo)

	return nil
}

func (t *Task) ExecuteNow(params TaskParams) (int, error) {
	if err := t.ValidateParams(params); err != nil {
		return 0, err
	}

	taskRun := &TaskRun{
		TaskName:  t.name,
		StartTime: time.Now().UTC(),
		Params:    params,
		Status:    "started",
	}

	err := t.blueBerry.db.SaveTaskRun(context.Background(), taskRun)
	if err != nil {
		fmt.Printf("unable to log task start: %v\n", err)
		return 0, err
	}

	go func(taskRun *TaskRun, params TaskParams) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.blueBerry.executing.Store(taskRun.ID, cancel)
		defer t.blueBerry.executing.Delete(taskRun.ID)

		logger := &Logger{taskRun: taskRun, db: t.blueBerry.db}
		err = t.taskFunc(ctx, params, logger)
		if err != nil {
			taskRun.Status = "failed"
			_ = logger.Error("Task failed due to: " + err.Error())
		} else {
			taskRun.Status = "completed"
		}
		taskRun.EndTime = time.Now().UTC()

		err = t.blueBerry.db.SaveTaskRun(ctx, taskRun)
		if err != nil {
			_ = logger.Error("Unable to save task run due to: " + err.Error())
		}
	}(taskRun, params)

	return taskRun.ID, nil
}

func (r *BlueBerry) storeSchedule(taskName string, scheduleInfo ScheduleInfo) {
	schedules, _ := r.schedules.LoadOrStore(taskName, []ScheduleInfo{})
	schedules = append(schedules.([]ScheduleInfo), scheduleInfo)
	r.schedules.Store(taskName, schedules)
}

func (r *BlueBerry) getSchedules(taskName string) []ScheduleInfo {
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

func (r *BlueBerry) InitTaskScheduler() {
	r.cron.Start()
}

func (r *BlueBerry) Shutdown() {
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

func (r *BlueBerry) CancelExecutionByID(executionID int) error {
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
