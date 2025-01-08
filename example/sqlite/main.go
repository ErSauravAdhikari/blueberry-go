package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	rasberry "github.com/ersauravadhikari/blueberry-go/blueberry"
	"github.com/ersauravadhikari/blueberry-go/blueberry/store"
)

var (
	task1Schema = rasberry.NewTaskSchema(rasberry.TaskParamDefinition{
		"param1": rasberry.TypeString,
		"param2": rasberry.TypeInt,
		"param3": rasberry.TypeBool,
	})
)

func task1(ctx context.Context, params rasberry.TaskParams, logger *rasberry.Logger) error {
	_ = logger.Info(fmt.Sprintf("The params are: %v", params))

	if err := logger.Info("Starting Task 1"); err != nil {
		return err
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ticker.C:
			if err := logger.Debug("Task 1 is still running..."); err != nil {
				return err
			}
		case <-timeout:
			select {
			case <-time.After(5 * time.Minute):
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
		case <-ctx.Done():
			if err := logger.Error("Task 1 cancelled"); err != nil {
				return err
			}
			return ctx.Err()
		}
	}
}

func main() {
	db, err := store.NewSQLiteDB("task_scheduler.db")
	if err != nil {
		log.Fatalf("Failed to initialize SQLite: %v", err)
	}
	defer db.Close()

	rb := rasberry.NewBlueBerryInstance(db)

	rb.AddWebOnlyPasswordAuth("admin", "password")
	rb.AddWebOnlyPasswordAuth("admin1", "password1")

	rb.AddAPIOnlyKeyAuth("key1", "Super Key 01")
	rb.AddAPIOnlyKeyAuth("key2", "Super Key 02")

	tsk1, err := rb.RegisterTask("task_1", task1, task1Schema)
	if err != nil {
		fmt.Printf("Failed to register task: %v\n", err)
		return
	}

	sc, err := tsk1.RegisterSchedule(rasberry.TaskParams{
		"param1": "value1",
		"param2": 1,
		"param3": true,
	}, "@every 1m")

	if err != nil {
		log.Fatalf("Failed to register schedule: %v", err)
	}

	// schedule var contains the schedule information
	fmt.Printf("Schedule with ID: %v has been registerd with CRON %s", sc.EntryID, sc.Schedule)

	_, err = tsk1.ExecuteNow(rasberry.TaskParams{
		"param1": "value1",
		"param2": 1,
		"param3": true,
	})
	if err != nil {
		log.Fatalf("Unable to execute right now")
	}

	// You can also remove the schedule dynamically.
	tsk1.DeleteSchedule(sc.EntryID) // Remove the registered schedule

	// Handle system signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v. Shutting down...", sig)
		rb.Shutdown()
		db.Close()
		os.Exit(0)
	}()

	rb.InitTaskScheduler()
	rb.RunAPI("8080")
}
