package main

import (
	"context"
	"fmt"
	rasberry "github.com/ersauravadhikari/raspberry-go/blueberry"
	"github.com/ersauravadhikari/raspberry-go/blueberry/store"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	select {
	case <-time.After(10 * time.Minute):
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

	rb.AddWebOnlyPasswordAuth("admin", "password")
	rb.AddWebOnlyPasswordAuth("admin1", "password1")

	rb.AddAPIOnlyKeyAuth("key1", "Super Key 01")
	rb.AddAPIOnlyKeyAuth("key2", "Super Key 02")

	tsk1, err := rb.RegisterTask("task_1", task1, task1Schema)
	if err != nil {
		fmt.Printf("Failed to register task: %v\n", err)
		return
	}

	// Can be executed directly from code without the scheduler as well
	//_, err = tsk1.ExecuteNow(rasberry.TaskParams{
	//	"param1": "value1",
	//	"param2": 1,
	//	"param3": true,
	//})
	//if err != nil {
	//	return
	//}

	if err := tsk1.RegisterSchedule(rasberry.TaskParams{
		"param1": "value1",
		"param2": 1,
		"param3": true,
	}, "@every 1m"); err != nil {
		log.Fatalf("Failed to register schedule: %v", err)
	}

	_, err = tsk1.ExecuteNow(rasberry.TaskParams{
		"param1": "value1",
		"param2": 1,
		"param3": true,
	})
	if err != nil {
		log.Fatalf("Unable to execute right now")
	}

	// Handle system signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v. Shutting down...", sig)
		rb.Shutdown()
		os.Exit(0)
	}()

	rb.InitTaskScheduler()
	rb.RunAPI("8080")
}
