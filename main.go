package main

import (
	"fmt"
	"user-rewards-service/db"
	"strings"
	"time"
)

/**
Scope
How we would be storing and crediting the reward points
How we would be defining the txn
Current points balance
Transaction history

Input
[
{
	contentId:,
    currWatchTime,
    timestamp
    totalRunningTime
    eventType: VOD_CONTENT_WATCH_TIME
},
{
   contentId:
   timestamp
   currentWatchTime
   eventType: LIVE_STREAM
}
]

interface
in memory db
  user rewards
    userId
    currentPointsBalance
  transactionHistory - map[string][]Transaction
    -transactionId
    - type - CREDIT/DEBIT
    - amount
    - contentId
    - timestamp
    - event
 func
 - creditPointsBalance
 -

Buildng the watch timeline tracking

1. how much the user has watched in today?
2. how much seconds did the user watch yesterday?
3. have a streacount on the users level
4.


UserWatchSnapshot
1. streakcount
1.1 totalContentCompleted
2. totalWatchedHours
3. lastWatchDuration
4. lastSeenTimestamp
5. WatchHistory[]

WatchHistoryLineItem
{
   contentId
   currentWatchTimeStamp
   totalRunningTime
   contentType
   timestamp
}

Service layer

UserRewardsService
 - process([]ConteWatchEvents)
 - getUserRewards
 - getUserWatchHistory

Interfaces for db
Have different interfaces for each domain of data we are dealing with
ContentDB - (if content is covered in the db layer)
WatchHistoryDB
UserDB
RewardsDB
*/

func main() {
	store := db.NewDB()

	seedEvents(store)

	for _, userId := range []string{"user-alice", "user-bob", "user-carol"} {
		printUserSummary(store, userId)
	}
}

// seedEvents pushes dummy content-watch events and corresponding reward credits
// for three users across multiple days, simulating realistic watch behaviour.
func seedEvents(store *db.DB) {
	today := time.Now().Truncate(24 * time.Hour)
	day := func(offset int) time.Time { return today.AddDate(0, 0, offset) }

	type event struct {
		userId           string
		contentId        string
		contentType      db.ContentType
		currentWatchTime int
		totalRunningTime int
		points           int
		timestamp        time.Time
	}

	events := []event{
		// Alice — 4-day streak, mix of VOD and live, some completed
		{"user-alice", "movie-inception", db.VOD, 7200, 7200, 50, day(-3)},
		{"user-alice", "movie-interstellar", db.VOD, 9000, 9720, 50, day(-3)},
		{"user-alice", "ipl-live-01", db.LiveStream, 3600, 0, 30, day(-2)},
		{"user-alice", "series-ep1", db.VOD, 2580, 2700, 20, day(-2)},
		{"user-alice", "series-ep2", db.VOD, 2700, 2700, 20, day(-1)},
		{"user-alice", "series-ep3", db.VOD, 1200, 2700, 10, day(-1)},
		{"user-alice", "ipl-live-02", db.LiveStream, 5400, 0, 40, day(0)},
		{"user-alice", "movie-dune", db.VOD, 8100, 9000, 50, day(0)},
		{"user-alice", "series-ep4", db.VOD, 2700, 2700, 20, day(0)},
		{"user-alice", "shorts-01", db.VOD, 120, 120, 5, day(0)},

		// Bob — streak broken by a gap on day -2, then resumed
		{"user-bob", "movie-avatar", db.VOD, 10800, 11400, 60, day(-5)},
		{"user-bob", "series-s1e1", db.VOD, 2700, 2700, 20, day(-5)},
		{"user-bob", "series-s1e2", db.VOD, 2700, 2700, 20, day(-4)},
		{"user-bob", "ipl-live-03", db.LiveStream, 7200, 0, 50, day(-4)},
		// gap on day -3 breaks the streak
		{"user-bob", "movie-oppenheimer", db.VOD, 10200, 10800, 60, day(-1)},
		{"user-bob", "series-s1e3", db.VOD, 1350, 2700, 10, day(-1)},
		{"user-bob", "ipl-live-04", db.LiveStream, 3600, 0, 30, day(0)},
		{"user-bob", "shorts-02", db.VOD, 90, 90, 5, day(0)},
		{"user-bob", "movie-parasite", db.VOD, 7740, 7740, 50, day(0)},
		{"user-bob", "series-s1e4", db.VOD, 2700, 2700, 20, day(0)},

		// Carol — light watcher, consistent short sessions every day
		{"user-carol", "shorts-03", db.VOD, 180, 180, 5, day(-4)},
		{"user-carol", "shorts-04", db.VOD, 240, 240, 5, day(-3)},
		{"user-carol", "series-comedy-e1", db.VOD, 1320, 1320, 15, day(-3)},
		{"user-carol", "ipl-live-05", db.LiveStream, 1800, 0, 20, day(-2)},
		{"user-carol", "shorts-05", db.VOD, 150, 150, 5, day(-2)},
		{"user-carol", "series-comedy-e2", db.VOD, 1320, 1320, 15, day(-1)},
		{"user-carol", "shorts-06", db.VOD, 90, 90, 5, day(-1)},
		{"user-carol", "movie-3idiots", db.VOD, 9720, 10080, 60, day(0)},
		{"user-carol", "ipl-live-06", db.LiveStream, 2700, 0, 25, day(0)},
		{"user-carol", "shorts-07", db.VOD, 120, 120, 5, day(0)},
	}

	for _, e := range events {
		store.RecordWatchEvent(e.userId, db.WatchHistoryLineItem{
			ContentId:        e.contentId,
			CurrentWatchTime: e.currentWatchTime,
			TotalRunningTime: e.totalRunningTime,
			ContentType:      e.contentType,
			Timestamp:        e.timestamp,
		})
		store.CreditPointsBalance(e.userId, e.points, db.Transaction{
			ContentId: e.contentId,
			Event:     string(e.contentType),
			Timestamp: e.timestamp,
		})
	}
}

func printUserSummary(store *db.DB, userId string) {
	divider := strings.Repeat("─", 60)
	fmt.Printf("\n%s\n USER: %s\n%s\n", divider, userId, divider)

	snap, err := store.GetWatchSnapshot(userId)
	if err != nil {
		fmt.Printf("  error fetching snapshot: %v\n", err)
		return
	}

	bal, _ := store.ShowPointsBalance(userId)
	watchedToday, _ := store.GetWatchedToday(userId)
	watchedYesterday, _ := store.GetWatchedYesterday(userId)

	fmt.Printf("  Points Balance       : %d pts\n", bal)
	fmt.Printf("  Streak               : %d day(s)\n", snap.StreakCount)
	fmt.Printf("  Content Completed    : %d\n", snap.TotalContentCompleted)
	fmt.Printf("  Total Watched        : %s\n", formatDuration(snap.TotalWatchedSeconds))
	fmt.Printf("  Watched Today        : %s\n", formatDuration(watchedToday))
	fmt.Printf("  Watched Yesterday    : %s\n", formatDuration(watchedYesterday))
	fmt.Printf("  Last Watch Duration  : %s\n", formatDuration(snap.LastWatchDuration))
	fmt.Printf("  Last Seen            : %s\n", snap.LastSeenTimestamp.Format("2006-01-02 15:04"))

	fmt.Printf("\n  Watch History (%d events):\n", len(snap.WatchHistory))
	fmt.Printf("  %-22s %-12s %-10s %-10s %s\n", "ContentId", "Type", "Watched", "Duration", "Date")
	fmt.Printf("  %s\n", strings.Repeat("·", 56))
	for _, item := range snap.WatchHistory {
		completed := " "
		if float64(item.CurrentWatchTime) >= 0.9*float64(item.TotalRunningTime) {
			completed = "✓"
		}
		duration := "live"
		if item.TotalRunningTime > 0 {
			duration = formatDuration(item.TotalRunningTime)
		}
		fmt.Printf("  %s %-21s %-12s %-10s %-10s %s\n",
			completed,
			item.ContentId,
			string(item.ContentType),
			formatDuration(item.CurrentWatchTime),
			duration,
			item.Timestamp.Format("2006-01-02"),
		)
	}

	txns, _ := store.GetTransactionHistory(userId)
	fmt.Printf("\n  Transactions (%d):\n", len(txns))
	fmt.Printf("  %-8s %-8s %-20s %s\n", "TxID", "Points", "ContentId", "Date")
	fmt.Printf("  %s\n", strings.Repeat("·", 48))
	for _, tx := range txns {
		fmt.Printf("  %-8s %-8d %-20s %s\n",
			tx.TransactionId,
			tx.Amount,
			tx.ContentId,
			tx.Timestamp.Format("2006-01-02"),
		)
	}
}

func formatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
