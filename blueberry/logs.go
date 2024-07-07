package rasberry

import (
	"context"
	"github.com/labstack/gommon/log"
	"time"
)

type Logger struct {
	taskRun *TaskRun
	db      DB
}

func (l *Logger) log(level, message string) error {
	logEntry := &TaskRunLog{
		TaskRunID: l.taskRun.ID,
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
	}
	err := l.db.SaveTaskRunLog(context.Background(), logEntry)
	if err != nil {
		return err
	}

	return nil
}

func (l *Logger) Info(message string) error {
	log.Info(message)
	return l.log("info", message)
}
func (l *Logger) Debug(message string) error {
	log.Debug(message)
	return l.log("debug", message)
}
func (l *Logger) Error(message string) error {
	log.Error(message)
	return l.log("error", message)
}
func (l *Logger) Success(message string) error {
	log.Info(message)
	return l.log("success", message)
}
