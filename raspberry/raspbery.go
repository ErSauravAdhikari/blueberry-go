package rasberry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type ScheduleInfo struct {
	Schedule      string                 `json:"schedule"`
	Params        map[string]interface{} `json:"params"`
	NextExecution int64                  `json:"next_execution_ts"`
	EntryID       cron.EntryID           `json:"-"`
}

type Task struct {
	name      string
	taskFunc  func(context.Context, map[string]interface{}, *Logger) error
	raspberry *Raspberry
	cancel    context.CancelFunc
}

type Raspberry struct {
	db        DB
	cron      *cron.Cron
	tasks     sync.Map
	taskMux   sync.Mutex
	schedules sync.Map // To store schedules per task
	executing sync.Map // To track currently executing tasks
}

func NewRaspberryInstance(db DB) *Raspberry {
	return &Raspberry{
		db:   db,
		cron: cron.New(),
	}
}

func (r *Raspberry) RegisterTask(taskName string, taskFunc func(context.Context, map[string]interface{}, *Logger) error) *Task {
	r.taskMux.Lock()
	defer r.taskMux.Unlock()
	r.tasks.Store(taskName, taskFunc)
	return &Task{
		name:      taskName,
		taskFunc:  taskFunc,
		raspberry: r,
	}
}

func (t *Task) RegisterSchedule(params map[string]interface{}, schedule string) error {
	_, ok := t.raspberry.tasks.Load(t.name)
	if !ok {
		return fmt.Errorf("task %s not found", t.name)
	}

	entryID, err := t.raspberry.cron.AddFunc(schedule, func() {
		taskRun := &TaskRun{
			TaskName:  t.name,
			StartTime: time.Now().UTC(),
			Params:    params,
			Status:    "status",
		}

		ctx, cancel := context.WithCancel(context.Background())
		t.cancel = cancel
		defer cancel()

		err := t.raspberry.db.SaveTaskRun(ctx, taskRun)
		if err != nil {
			// Log the error but continue to run the task
			fmt.Printf("Unable to log task start: %v\n", err)
		}

		// Track the executing task
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
