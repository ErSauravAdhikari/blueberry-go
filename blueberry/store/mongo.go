package store

import (
	"context"

	blueberry "github.com/ersauravadhikari/blueberry-go/blueberry"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB is a struct that provides MongoDB client and collections for task management.
type MongoDB struct {
	client      *mongo.Client
	database    *mongo.Database
	taskRuns    *mongo.Collection
	taskRunLogs *mongo.Collection
}

// NewMongoDB initializes a new MongoDB instance, connects to the database, and sets up collections and indexes.
func NewMongoDB(uri, dbName string) (*MongoDB, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	taskRuns := db.Collection("task_runs")
	taskRunLogs := db.Collection("task_run_logs")

	// Initialize MongoDB instance
	mongoDB := &MongoDB{
		client:      client,
		database:    db,
		taskRuns:    taskRuns,
		taskRunLogs: taskRunLogs,
	}

	// Initialize counters for taskRunID and taskRunLogID
	if err := mongoDB.InitializeCounters(context.Background()); err != nil {
		return nil, err
	}

	// Set indexes for taskRuns and taskRunLogs collections
	if err := mongoDB.createIndexes(); err != nil {
		return nil, err
	}

	return mongoDB, nil
}

// InitializeCounters initializes necessary counters for the task run and log ID sequences.
func (db *MongoDB) InitializeCounters(ctx context.Context) error {
	if err := db.ensureCounter(ctx, "taskRunID"); err != nil {
		return err
	}
	return db.ensureCounter(ctx, "taskRunLogID")
}

// ensureCounter checks if a counter document exists, initializing it if missing.
func (db *MongoDB) ensureCounter(ctx context.Context, counterName string) error {
	filter := bson.D{{Key: "_id", Value: counterName}}
	update := bson.D{{Key: "$setOnInsert", Value: bson.D{{Key: "seq_value", Value: 1}}}}
	options := options.Update().SetUpsert(true)

	_, err := db.database.Collection("counters").UpdateOne(ctx, filter, update, options)
	return err
}

// createIndexes creates indexes on frequently queried fields for optimal performance.
func (db *MongoDB) createIndexes() error {
	// Index for task_runs collection on 'taskname' and 'starttime'
	taskRunIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "taskname", Value: 1},
			{Key: "starttime", Value: -1},
		},
		Options: options.Index().SetBackground(true),
	}
	if _, err := db.taskRuns.Indexes().CreateOne(context.Background(), taskRunIndex); err != nil {
		return err
	}

	// Index for task_run_logs collection on 'taskrunid' and 'level'
	taskRunLogIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "taskrunid", Value: 1},
			{Key: "level", Value: 1},
		},
		Options: options.Index().SetBackground(true),
	}
	if _, err := db.taskRunLogs.Indexes().CreateOne(context.Background(), taskRunLogIndex); err != nil {
		return err
	}

	return nil
}

// GetNextSequence increments and returns the next sequence value for a given counter name.
func (db *MongoDB) GetNextSequence(ctx context.Context, name string) (int, error) {
	filter := bson.M{"_id": name}
	update := bson.M{"$inc": bson.M{"seq_value": 1}}
	options := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var counter struct {
		SeqValue int `bson:"seq_value"`
	}
	err := db.database.Collection("counters").FindOneAndUpdate(ctx, filter, update, options).Decode(&counter)
	if err != nil {
		return 0, err
	}
	return counter.SeqValue, nil
}

// SaveTaskRun inserts a new task run document or updates an existing one.
func (db *MongoDB) SaveTaskRun(ctx context.Context, taskRun *blueberry.TaskRun) error {
	if taskRun.ID == 0 {
		nextID, err := db.GetNextSequence(ctx, "taskRunID")
		if err != nil {
			return err
		}
		taskRun.ID = nextID

		// Insert new task run
		_, err = db.taskRuns.InsertOne(ctx, taskRun)
		return err
	}

	// Update existing task run
	filter := bson.M{"id": taskRun.ID}
	update := bson.M{"$set": taskRun}
	_, err := db.taskRuns.UpdateOne(ctx, filter, update)
	return err
}

// SaveTaskRunLog inserts a new task run log document with auto-incremented ID.
func (db *MongoDB) SaveTaskRunLog(ctx context.Context, taskRunLog *blueberry.TaskRunLog) error {
	nextID, err := db.GetNextSequence(ctx, "taskRunLogID")
	if err != nil {
		return err
	}
	taskRunLog.ID = nextID

	_, err = db.taskRunLogs.InsertOne(ctx, taskRunLog)
	return err
}

// GetTaskRuns retrieves all task runs.
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

// GetPaginatedTaskRunsForTaskName retrieves task runs for a specific task, paginated and sorted by start time.
func (db *MongoDB) GetPaginatedTaskRunsForTaskName(ctx context.Context, name string, page, limit int) ([]blueberry.TaskRun, error) {
	skip := (page - 1) * limit
	cursor, err := db.taskRuns.Find(ctx, bson.M{"taskname": name}, options.Find().SetSort(bson.M{"starttime": -1}).SetSkip(int64(skip)).SetLimit(int64(limit)))
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

// GetTaskRunsCountForTaskName returns the count of task runs for a specific task name.
func (db *MongoDB) GetTaskRunsCountForTaskName(ctx context.Context, name string) (int, error) {
	count, err := db.taskRuns.CountDocuments(ctx, bson.M{"taskname": name})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetTaskRunLogs retrieves all logs for a specific task run ID.
func (db *MongoDB) GetTaskRunLogs(ctx context.Context, taskRunID int) ([]blueberry.TaskRunLog, error) {
	cursor, err := db.taskRunLogs.Find(ctx, bson.M{"taskrunid": taskRunID})
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

// GetPaginatedTaskRunLogs retrieves paginated task run logs for a specific task run ID and level filter.
func (db *MongoDB) GetPaginatedTaskRunLogs(ctx context.Context, taskRunID int, level string, page, size int) ([]blueberry.TaskRunLog, int, error) {
	filter := bson.M{"taskrunid": taskRunID}
	if level != "all" {
		filter["level"] = level
	}

	// Sort by 'starttime' in descending order so that we get the latest logs first
	options := options.Find().SetLimit(int64(size)).SetSkip(int64((page - 1) * size)).SetSort(bson.M{"starttime": -1})
	cursor, err := db.taskRunLogs.Find(ctx, filter, options)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var taskRunLogs []blueberry.TaskRunLog
	for cursor.Next(ctx) {
		var taskRunLog blueberry.TaskRunLog
		if err := cursor.Decode(&taskRunLog); err != nil {
			return nil, 0, err
		}
		taskRunLogs = append(taskRunLogs, taskRunLog)
	}

	totalCount, err := db.taskRunLogs.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return taskRunLogs, int(totalCount), nil
}

// GetTaskRunByID retrieves a specific task run by ID.
func (db *MongoDB) GetTaskRunByID(ctx context.Context, id int) (*blueberry.TaskRun, error) {
	filter := bson.M{"id": id}
	var taskRun blueberry.TaskRun
	err := db.taskRuns.FindOne(ctx, filter).Decode(&taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

// Close disconnects the MongoDB client.
func (db *MongoDB) Close() error {
	return db.client.Disconnect(context.Background())
}
