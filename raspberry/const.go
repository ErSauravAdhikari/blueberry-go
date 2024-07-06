package rasberry

const (
	// Common cron intervals
	RunEveryMinute    = "@every 1m"
	RunEvery5Minutes  = "@every 5m"
	RunEvery10Minutes = "@every 10m"
	RunEvery15Minutes = "@every 15m"
	RunEvery30Minutes = "@every 30m"
	RunEveryHour      = "@every 1h"
	RunEvery2Hours    = "@every 2h"
	RunEvery3Hours    = "@every 3h"
	RunEvery4Hours    = "@every 4h"
	RunEvery6Hours    = "@every 6h"
	RunEvery12Hours   = "@every 12h"
	RunEveryDay       = "@every 24h"
	RunEveryWeek      = "@every 168h" // 7 * 24 hours

	// Specific time of day (example cron expressions)
	RunAtMidnight = "0 0 * * *"
	RunAtNoon     = "0 12 * * *"
	RunAt6AM      = "0 6 * * *"
	RunAt6PM      = "0 18 * * *"

	// Specific days of the week
	RunEveryMondayAtNoon     = "0 12 * * 1"
	RunEveryFridayAtNoon     = "0 12 * * 5"
	RunEverySundayAtMidnight = "0 0 * * 0"
)
