package blueberry

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/gommon/log"
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
	if err := l.db.SaveTaskRunLog(context.Background(), logEntry); err != nil {
		return fmt.Errorf("failed to save log entry: %w", err)
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

func (l *Logger) Infof(message string, args ...any) error {
	msg := fmt.Sprintf(message, args...)
	log.Info(msg)
	return l.log("info", msg)
}

func (l *Logger) Debugf(message string, args ...any) error {
	msg := fmt.Sprintf(message, args...)
	log.Debug(msg)
	return l.log("debug", msg)
}

func (l *Logger) Errorf(message string, args ...any) error {
	msg := fmt.Sprintf(message, args...)
	log.Error(msg)
	return l.log("error", msg)
}

func (l *Logger) Successf(message string, args ...any) error {
	msg := fmt.Sprintf(message, args...)
	log.Info(msg)
	return l.log("success", msg)
}
