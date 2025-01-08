# Understanding Schedule ID Management in BlueBerry
## The Problem with Cron Entry IDs

### Basic Understanding
When you create a schedule in BlueBerry, internally it uses a cron system that assigns an `EntryID` (an integer). For example:

```go
schedule, err := task.RegisterSchedule(params, "@every 1m")
// schedule.EntryID might be 1
```

### The Issue with Restarts
When your application restarts:
1. The cron system reinitializes
2. Entry IDs are reassigned sequentially
3. Previous Entry IDs are lost

Example scenario:
```go
// First run of application
schedule1 -> EntryID: 1
schedule2 -> EntryID: 2
schedule3 -> EntryID: 3

// After restart
schedule1 -> EntryID: 1 
schedule2 -> EntryID: 3  // Same ID but different schedule!
schedule3 -> EntryID: 2  // Same ID but different schedule!
```

This creates problems:
1. Users might have stored the old EntryID
2. API calls using old IDs would affect wrong schedules
3. No persistence of schedule identity

## The Solution: ID Mapping

### Basic Implementation

```go
type ScheduleMapping struct {
    PublicID    string       // Stable UUID for external use
    EntryID     cron.EntryID // Current cron entry ID (changes on restart)
    TaskName    string
    CreatedAt   time.Time
}

type ScheduleStore struct {
    mapping     map[string]ScheduleMapping  // PublicID to Mapping
    entryToID   map[cron.EntryID]string    // EntryID to PublicID
    mu          sync.RWMutex
}
```

### Usage Example

```go
// Creating a new schedule
func (s *ScheduleStore) CreateSchedule(task *blueberry.Task, params blueberry.TaskParams, schedule string) (string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Create schedule in cron
    cronSchedule, err := task.RegisterSchedule(params, schedule)
    if err != nil {
        return "", err
    }

    // Generate stable public ID
    publicID := uuid.New().String()

    // Store mapping
    s.mapping[publicID] = ScheduleMapping{
        PublicID:  publicID,
        EntryID:   cronSchedule.EntryID,
        TaskName:  task.Name,
        CreatedAt: time.Now(),
    }
    s.entryToID[cronSchedule.EntryID] = publicID

    return publicID, nil
}
```

## Complete Tutorial

### Step 1: Setting up the Schedule Store

```go
type ScheduleStore struct {
    db          *sql.DB
    mapping     map[string]ScheduleMapping
    entryToID   map[cron.EntryID]string
    mu          sync.RWMutex
    bb          *blueberry.BlueBerry
}

func NewScheduleStore(db *sql.DB, bb *blueberry.BlueBerry) *ScheduleStore {
    return &ScheduleStore{
        db:        db,
        mapping:   make(map[string]ScheduleMapping),
        entryToID: make(map[cron.EntryID]string),
        bb:        bb,
    }
}
```

### Step 2: Implementing Basic Operations

```go
// Create a new schedule
func (s *ScheduleStore) Create(taskName string, params blueberry.TaskParams, schedule string) (string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Start transaction
    tx, err := s.db.Begin()
    if err != nil {
        return "", err
    }
    defer tx.Rollback()

    // Generate public ID
    publicID := uuid.New().String()

    // Create schedule
    task := s.bb.GetTask(taskName)
    cronSchedule, err := task.RegisterSchedule(params, schedule)
    if err != nil {
        return "", err
    }

    // Store in database
    if err := s.storeScheduleInDB(tx, publicID, taskName, params, schedule, cronSchedule.EntryID); err != nil {
        return "", err
    }

    // Update mapping
    s.updateMapping(publicID, cronSchedule.EntryID, taskName)

    return publicID, tx.Commit()
}
```

### Step 3: Handling Restarts

```go
func (s *ScheduleStore) Initialize() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Load all schedules from database
    schedules, err := s.loadSchedulesFromDB()
    if err != nil {
        return err
    }

    // Recreate each schedule
    for _, stored := range schedules {
        task := s.bb.GetTask(stored.TaskName)
        if task == nil {
            log.Printf("Task %s no longer exists", stored.TaskName)
            continue
        }

        // Recreate in cron
        cronSchedule, err := task.RegisterSchedule(stored.Params, stored.Schedule)
        if err != nil {
            log.Printf("Failed to recreate schedule %s: %v", stored.PublicID, err)
            continue
        }

        // Update mapping with new entry ID
        s.updateMapping(stored.PublicID, cronSchedule.EntryID, stored.TaskName)
    }

    return nil
}
```

### Step 4: Thread-Safe Operations

```go
func (s *ScheduleStore) GetByPublicID(publicID string) (*ScheduleMapping, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    mapping, exists := s.mapping[publicID]
    if !exists {
        return nil, errors.New("schedule not found")
    }

    return &mapping, nil
}

func (s *ScheduleStore) GetByEntryID(entryID cron.EntryID) (*ScheduleMapping, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    publicID, exists := s.entryToID[entryID]
    if !exists {
        return nil, errors.New("schedule not found")
    }

    mapping, exists := s.mapping[publicID]
    if !exists {
        return nil, errors.New("inconsistent mapping state")
    }

    return &mapping, nil
}
```

## Advanced Use Cases

### 1. Schedule State Management

```go
type ScheduleState struct {
    PublicID    string
    Status      string    // "active", "paused", "error"
    LastRun     time.Time
    NextRun     time.Time
    RunCount    int64
    LastError   string
}
```

### 2. Schedule History

```go
type ScheduleExecution struct {
    PublicID    string
    StartTime   time.Time
    EndTime     time.Time
    Status      string
    Error       string
    Params      map[string]interface{}
}
```

### 3. Schedule Groups

```go
type ScheduleGroup struct {
    GroupID     string
    Name        string
    Schedules   []string  // Public IDs
    Priority    int
}
```

### 4. Schedule Dependencies

```go
type ScheduleDependency struct {
    ScheduleID      string
    DependsOn       []string  // Public IDs
    WaitPolicy      string    // "all", "any", "majority"
    TimeoutMinutes  int
}
```

### 5. Dynamic Schedule Updates

```go
type DynamicSchedule struct {
    PublicID    string
    Expression  string
    Rules       []ScheduleRule
    LastUpdate  time.Time
}

type ScheduleRule struct {
    Condition   string  // e.g., "load > 80%"
    Action      string  // e.g., "delay 5m"
}
```