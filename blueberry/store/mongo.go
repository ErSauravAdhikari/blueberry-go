package store

import (
	"context"
	blueberry "github.com/ersauravadhikari/blueberry-go/blueberry"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	client      *mongo.Client
	database    *mongo.Database
	taskRuns    *mongo.Collection
	taskRunLogs *mongo.Collection
}

func NewMongoDB(uri, dbName string) (*MongoDB, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	taskRuns := db.Collection("task_runs")
	taskRunLogs := db.Collection("task_run_logs")

	return &MongoDB{
		client:      client,
		database:    db,
		taskRuns:    taskRuns,
		taskRunLogs: taskRunLogs,
	}, nil
}

func (db *MongoDB) SaveTaskRun(ctx context.Context, taskRun *blueberry.TaskRun) error {
	if taskRun.ID == 0 {
		result, err := db.taskRuns.InsertOne(ctx, taskRun)
		if err != nil {
			return err
		}
		taskRun.ID = int(result.InsertedID.(primitive.ObjectID).Timestamp().Unix())
	} else {
		filter := bson.M{"id": taskRun.ID}
		update := bson.M{"$set": taskRun}
		_, err := db.taskRuns.UpdateOne(ctx, filter, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *MongoDB) SaveTaskRunLog(ctx context.Context, taskRunLog *blueberry.TaskRunLog) error {
	result, err := db.taskRunLogs.InsertOne(ctx, taskRunLog)
	if err != nil {
		return err
	}
	taskRunLog.ID = int(result.InsertedID.(primitive.ObjectID).Timestamp().Unix())
	return nil
}

func (db *MongoDB) GetTaskRuns(ctx context.Context) ([]blueberry.TaskRun, error) {
	cursor, err := db.taskRuns.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var taskRuns []blueberry.TaskRun
	for cursor.Next(ctx) {
		var taskRun blueberry.TaskRun
		if err := cursor.Decode(&taskRun); err != nil {
			return nil, err
		}
		taskRuns = append(taskRuns, taskRun)
	}
	return taskRuns, nil
}

func (db *MongoDB) GetPaginatedTaskRunsForTaskName(ctx context.Context, name string, page, limit int) ([]blueberry.TaskRun, error) {
	skip := (page - 1) * limit
	cursor, err := db.taskRuns.Find(ctx, bson.M{"task_name": name}, options.Find().SetSort(bson.M{"start_time": -1}).SetSkip(int64(skip)).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var taskRuns []blueberry.TaskRun
	for cursor.Next(ctx) {
		var taskRun blueberry.TaskRun
		if err := cursor.Decode(&taskRun); err != nil {
			return nil, err
		}
		taskRuns = append(taskRuns, taskRun)
	}
	return taskRuns, nil
}

func (db *MongoDB) GetTaskRunsCountForTaskName(ctx context.Context, name string) (int, error) {
	count, err := db.taskRuns.CountDocuments(ctx, bson.M{"task_name": name})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (db *MongoDB) GetTaskRunLogs(ctx context.Context, taskRunID int) ([]blueberry.TaskRunLog, error) {
	filter := bson.M{"task_run_id": taskRunID}
	cursor, err := db.taskRunLogs.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var taskRunLogs []blueberry.TaskRunLog
	for cursor.Next(ctx) {
		var taskRunLog blueberry.TaskRunLog
		if err := cursor.Decode(&taskRunLog); err != nil {
			return nil, err
		}
		taskRunLogs = append(taskRunLogs, taskRunLog)
	}
	return taskRunLogs, nil
}

func (db *MongoDB) GetPaginatedTaskRunLogs(ctx context.Context, taskRunID int, level string, page, size int) ([]blueberry.TaskRunLog, error) {
	filter := bson.M{"task_run_id": taskRunID}
	if level != "all" {
		filter["level"] = level
	}

	options := options.Find()
	options.SetLimit(int64(size))
	options.SetSkip(int64((page - 1) * size))

	cursor, err := db.taskRunLogs.Find(ctx, filter, options)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var taskRunLogs []blueberry.TaskRunLog
	for cursor.Next(ctx) {
		var taskRunLog blueberry.TaskRunLog
		if err := cursor.Decode(&taskRunLog); err != nil {
			return nil, err
		}
		taskRunLogs = append(taskRunLogs, taskRunLog)
	}
	return taskRunLogs, nil
}

func (db *MongoDB) GetTaskRunByID(ctx context.Context, id int) (*blueberry.TaskRun, error) {
	filter := bson.M{"id": id}
	var taskRun blueberry.TaskRun
	err := db.taskRuns.FindOne(ctx, filter).Decode(&taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

func (db *MongoDB) Close() error {
	return db.client.Disconnect(context.Background())
}
