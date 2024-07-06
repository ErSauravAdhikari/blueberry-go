package main

import (
	"context"
	rasberry "github.com/ersauravadhikari/raspberry-go/raspberry"
	"github.com/ersauravadhikari/raspberry-go/raspberry/store"
	"log"
	"time"
)

func task1(ctx context.Context, params map[string]interface{}, logger *rasberry.Logger) error {
	if err := logger.Info("Starting Task 1"); err != nil {
		return err
	}
	select {
	case <-time.After(10 * time.Second):
		if err := logger.Success("Task 1 completed successfully"); err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		if err := logger.Error("Task 1 cancelled"); err != nil {
			return err
		}
		return ctx.Err()
	}
}
func main() {
	db, err := store.NewSQLiteDB("task_scheduler.db")
	if err != nil {
		log.Fatalf("Failed to initialize SQLite: %v", err)
	}
	defer db.Close()

	rb := rasberry.NewRaspberryInstance(db)

	tsk1 := rb.RegisterTask("task_1", task1)
	if err := tsk1.RegisterSchedule(map[string]interface{}{"param1": "value1"}, "@every 1m"); err != nil {
		log.Fatalf("Failed to register schedule: %v", err)
	}
	if err := tsk1.RegisterSchedule(map[string]interface{}{"param1": "value2"}, "@every 5m"); err != nil {
		log.Fatalf("Failed to register schedule: %v", err)
	}

	tsk2 := rb.RegisterTask("task_2", task1)
	if err := tsk2.RegisterSchedule(map[string]interface{}{}, rasberry.RunEveryMinute); err != nil {
		log.Fatalf("Failed to register schedule: %v", err)
	}

	rb.InitTaskScheduler()
	rb.RunAPI("8080")
}
