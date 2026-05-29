package db

import (
	"sync"
	"testing"
	"time"
)

func TestCreditAndShowBalance(t *testing.T) {
	store := NewDB()

	err := store.CreditPointsBalance("u1", 100, Transaction{ContentId: "c1", Event: "VOD_CONTENT_WATCH_TIME"})
	if err != nil {
		t.Fatal(err)
	}
	err = store.CreditPointsBalance("u1", 50, Transaction{ContentId: "c2", Event: "VOD_CONTENT_WATCH_TIME"})
	if err != nil {
		t.Fatal(err)
	}

	bal, err := store.ShowPointsBalance("u1")
	if err != nil {
		t.Fatal(err)
	}
	if bal != 150 {
		t.Errorf("expected 150, got %d", bal)
	}
}

func TestShowBalanceUnknownUser(t *testing.T) {
	store := NewDB()
	_, err := store.ShowPointsBalance("nobody")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestTransactionHistoryOrder(t *testing.T) {
	store := NewDB()

	store.CreditPointsBalance("u1", 10, Transaction{ContentId: "c1"})
	store.CreditPointsBalance("u1", 20, Transaction{ContentId: "c2"})
	store.CreditPointsBalance("u1", 30, Transaction{ContentId: "c3"})

	history, err := store.GetTransactionHistory("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 transactions, got %d", len(history))
	}
	if history[0].Amount != 10 || history[1].Amount != 20 || history[2].Amount != 30 {
		t.Error("transaction history not in insertion order")
	}
	// IDs should be unique and sequential
	if history[0].TransactionId == history[1].TransactionId {
		t.Error("transaction IDs must be unique")
	}
}

func TestRecordWatchEventUpdatesSnapshot(t *testing.T) {
	store := NewDB()
	now := time.Now()

	item := WatchHistoryLineItem{
		ContentId:        "c1",
		CurrentWatchTime: 900,
		TotalRunningTime: 1800,
		ContentType:      VOD,
		Timestamp:        now,
	}
	if err := store.RecordWatchEvent("u1", item); err != nil {
		t.Fatal(err)
	}

	snap, err := store.GetWatchSnapshot("u1")
	if err != nil {
		t.Fatal(err)
	}
	if snap.LastWatchDuration != 900 {
		t.Errorf("expected LastWatchDuration 900, got %d", snap.LastWatchDuration)
	}
	if !snap.LastSeenTimestamp.Equal(now) {
		t.Error("LastSeenTimestamp not updated")
	}
	if snap.TotalWatchedSeconds != 900 {
		t.Errorf("expected TotalWatchedSeconds 900, got %d", snap.TotalWatchedSeconds)
	}
}

func TestContentCompletionThreshold(t *testing.T) {
	store := NewDB()
	now := time.Now()

	// 89% — should NOT count as completed
	store.RecordWatchEvent("u1", WatchHistoryLineItem{
		ContentId:        "c1",
		CurrentWatchTime: 89,
		TotalRunningTime: 100,
		ContentType:      VOD,
		Timestamp:        now,
	})
	snap, _ := store.GetWatchSnapshot("u1")
	if snap.TotalContentCompleted != 0 {
		t.Errorf("expected 0 completions, got %d", snap.TotalContentCompleted)
	}

	// 90% — should count as completed
	store.RecordWatchEvent("u1", WatchHistoryLineItem{
		ContentId:        "c2",
		CurrentWatchTime: 90,
		TotalRunningTime: 100,
		ContentType:      VOD,
		Timestamp:        now,
	})
	snap, _ = store.GetWatchSnapshot("u1")
	if snap.TotalContentCompleted != 1 {
		t.Errorf("expected 1 completion, got %d", snap.TotalContentCompleted)
	}
}

func TestGetWatchedToday(t *testing.T) {
	store := NewDB()
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	store.RecordWatchEvent("u1", WatchHistoryLineItem{ContentId: "c1", CurrentWatchTime: 300, TotalRunningTime: 600, ContentType: VOD, Timestamp: now})
	store.RecordWatchEvent("u1", WatchHistoryLineItem{ContentId: "c2", CurrentWatchTime: 200, TotalRunningTime: 600, ContentType: VOD, Timestamp: now})
	store.RecordWatchEvent("u1", WatchHistoryLineItem{ContentId: "c3", CurrentWatchTime: 100, TotalRunningTime: 600, ContentType: VOD, Timestamp: yesterday})

	today, err := store.GetWatchedToday("u1")
	if err != nil {
		t.Fatal(err)
	}
	if today != 500 {
		t.Errorf("expected 500s today, got %d", today)
	}

	yest, err := store.GetWatchedYesterday("u1")
	if err != nil {
		t.Fatal(err)
	}
	if yest != 100 {
		t.Errorf("expected 100s yesterday, got %d", yest)
	}
}

func TestStreakConsecutiveDays(t *testing.T) {
	store := NewDB()

	day1 := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC) // gap — breaks streak

	store.RecordWatchEvent("u1", WatchHistoryLineItem{ContentId: "c1", CurrentWatchTime: 60, TotalRunningTime: 600, ContentType: VOD, Timestamp: day1})
	store.RecordWatchEvent("u1", WatchHistoryLineItem{ContentId: "c2", CurrentWatchTime: 60, TotalRunningTime: 600, ContentType: VOD, Timestamp: day2})

	snap, _ := store.GetWatchSnapshot("u1")
	if snap.StreakCount != 2 {
		t.Errorf("expected streak 2, got %d", snap.StreakCount)
	}

	store.RecordWatchEvent("u1", WatchHistoryLineItem{ContentId: "c3", CurrentWatchTime: 60, TotalRunningTime: 600, ContentType: VOD, Timestamp: day3})
	snap, _ = store.GetWatchSnapshot("u1")
	if snap.StreakCount != 1 {
		t.Errorf("expected streak reset to 1, got %d", snap.StreakCount)
	}
}

func TestConcurrentCredits(t *testing.T) {
	store := NewDB()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.CreditPointsBalance("u1", 1, Transaction{ContentId: "c1"})
		}()
	}
	wg.Wait()

	bal, err := store.ShowPointsBalance("u1")
	if err != nil {
		t.Fatal(err)
	}
	if bal != 100 {
		t.Errorf("expected balance 100 after concurrent credits, got %d", bal)
	}
}
