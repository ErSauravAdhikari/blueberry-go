package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	rasberry "github.com/ersauravadhikari/raspberry-go/raspberry"
	"github.com/ersauravadhikari/raspberry-go/raspberry/store"
)

func task1(ctx context.Context, params map[string]interface{}, logger *rasberry.Logger) error {
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
	mongoDB, err := store.NewMongoDB("mongodb://localhost:27017", "task_scheduler")
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}
	defer mongoDB.Close()

	rb := rasberry.NewRaspberryInstance(mongoDB)

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
