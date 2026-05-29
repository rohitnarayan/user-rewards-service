package db

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type TransactionType string

const (
	Credit TransactionType = "CREDIT"
)

type ContentType string

const (
	VOD        ContentType = "VOD"
	LiveStream ContentType = "LIVE_STREAM"
)

type Transaction struct {
	TransactionId string
	Type          TransactionType
	Amount        int
	ContentId     string
	Timestamp     time.Time
	Event         string
}

type UserReward struct {
	CurrentPointsBalance int
}

type WatchHistoryLineItem struct {
	ContentId        string
	CurrentWatchTime int // seconds watched so far (progress position)
	TotalRunningTime int // total content duration in seconds
	ContentType      ContentType
	Timestamp        time.Time
}

type UserWatchSnapshot struct {
	UserId                string
	StreakCount           int
	TotalContentCompleted int
	TotalWatchedSeconds   int // all-time cumulative seconds watched
	LastWatchDuration     int // seconds watched in the last event
	LastSeenTimestamp     time.Time
	WatchHistory          []WatchHistoryLineItem
}

type DB struct {
	UserRewards        map[string]*UserReward
	TransactionHistory map[string][]*Transaction
	UserWatchSnapshots map[string]*UserWatchSnapshot
	txCounter          int
	mu                 sync.RWMutex
}

func NewDB() *DB {
	return &DB{
		UserRewards:        make(map[string]*UserReward),
		TransactionHistory: make(map[string][]*Transaction),
		UserWatchSnapshots: make(map[string]*UserWatchSnapshot),
	}
}

func (db *DB) CreditPointsBalance(userId string, amount int, tx Transaction) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.UserRewards[userId]; !exists {
		db.UserRewards[userId] = &UserReward{}
	}

	db.txCounter++
	tx.TransactionId = fmt.Sprintf("tx-%d", db.txCounter)
	tx.Type = Credit
	tx.Amount = amount

	db.UserRewards[userId].CurrentPointsBalance += amount
	db.TransactionHistory[userId] = append(db.TransactionHistory[userId], &tx)
	return nil
}

func (db *DB) ShowPointsBalance(userId string) (int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	reward, exists := db.UserRewards[userId]
	if !exists {
		return 0, errors.New("user not found")
	}
	return reward.CurrentPointsBalance, nil
}

func (db *DB) GetTransactionHistory(userId string) ([]*Transaction, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if _, exists := db.UserRewards[userId]; !exists {
		return nil, errors.New("user not found")
	}
	return db.TransactionHistory[userId], nil
}

func (db *DB) RecordWatchEvent(userId string, item WatchHistoryLineItem) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	snapshot, exists := db.UserWatchSnapshots[userId]
	if !exists {
		snapshot = &UserWatchSnapshot{UserId: userId}
		db.UserWatchSnapshots[userId] = snapshot
	}

	snapshot.WatchHistory = append(snapshot.WatchHistory, item)
	snapshot.TotalWatchedSeconds += item.CurrentWatchTime
	snapshot.LastWatchDuration = item.CurrentWatchTime
	snapshot.LastSeenTimestamp = item.Timestamp

	if float64(item.CurrentWatchTime) >= 0.9*float64(item.TotalRunningTime) {
		snapshot.TotalContentCompleted++
	}

	db.updateStreak(snapshot, item.Timestamp)
	return nil
}

// updateStreak increments the streak if the previous watch was on the prior calendar day,
// resets to 1 if there was a gap, and does nothing if the user already has an event today.
func (db *DB) updateStreak(snapshot *UserWatchSnapshot, eventTime time.Time) {
	eventDay := eventTime.Truncate(24 * time.Hour)

	if len(snapshot.WatchHistory) <= 1 {
		snapshot.StreakCount = 1
		return
	}

	prev := snapshot.WatchHistory[len(snapshot.WatchHistory)-2]
	prevDay := prev.Timestamp.Truncate(24 * time.Hour)

	switch {
	case eventDay.Equal(prevDay):
		// same day — streak unchanged
	case eventDay.Equal(prevDay.Add(24 * time.Hour)):
		// consecutive day — extend streak
		snapshot.StreakCount++
	default:
		// gap — reset
		snapshot.StreakCount = 1
	}
}

func (db *DB) GetWatchedToday(userId string) (int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return db.sumWatchTimeForDay(userId, time.Now())
}

func (db *DB) GetWatchedYesterday(userId string) (int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return db.sumWatchTimeForDay(userId, time.Now().AddDate(0, 0, -1))
}

func (db *DB) sumWatchTimeForDay(userId string, day time.Time) (int, error) {
	snapshot, exists := db.UserWatchSnapshots[userId]
	if !exists {
		return 0, errors.New("user not found")
	}

	target := day.Truncate(24 * time.Hour)
	total := 0
	for _, item := range snapshot.WatchHistory {
		if item.Timestamp.Truncate(24 * time.Hour).Equal(target) {
			total += item.CurrentWatchTime
		}
	}
	return total, nil
}

func (db *DB) GetWatchSnapshot(userId string) (*UserWatchSnapshot, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	snapshot, exists := db.UserWatchSnapshots[userId]
	if !exists {
		return nil, errors.New("user not found")
	}
	return snapshot, nil
}
