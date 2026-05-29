# Claude Code Project Context

## Project Overview
A Go rewards and watch-tracking system for a streaming platform. Tracks user reward points, transaction history, watch history, streaks, and content completion.

---

## Architecture

### Packages

```
user-rewards-service/
├── main.go                        — wiring + seed data demo
├── db/
│   ├── db.go                      — in-memory DB (all data structures + methods)
│   ├── interfaces.go              — WatchDB, RewardsDB interfaces (planned)
│   └── db_test.go                 — unit + race-detector tests
├── config/
│   └── rules.go                   — reward rules + RewardConfig (planned)
├── service/
│   └── user_rewards_service.go    — UserRewardsService (planned)
└── docs/plans/PLAN.md             — detailed implementation plan
```

---

## DB Layer (`db/db.go`)

### Key Structs

| Struct | Key Fields |
|---|---|
| `Transaction` | `TransactionId`, `Type`, `Amount`, `ContentId`, `Timestamp`, `Event` |
| `UserReward` | `CurrentPointsBalance int` |
| `WatchHistoryLineItem` | `ContentId`, `CurrentWatchTime int` (seconds), `TotalRunningTime int`, `ContentType`, `Timestamp` |
| `UserWatchSnapshot` | `UserId`, `StreakCount`, `HighestStreak`*, `TotalContentCompleted`, `TotalWatchedSeconds`, `LastWatchDuration`, `LastSeenTimestamp`, `WatchHistory []WatchHistoryLineItem` |
| `DB` | `UserRewards map[string]*UserReward`, `TransactionHistory map[string][]*Transaction`, `UserWatchSnapshots map[string]*UserWatchSnapshot`, `mu sync.RWMutex` |

\* `HighestStreak` is planned but not yet added to the struct.

### Constants
- `TransactionType`: `Credit = "CREDIT"`
- `ContentType`: `VOD = "VOD"`, `LiveStream = "LIVE_STREAM"`

### DB Methods
- `NewDB() *DB`
- `CreditPointsBalance(userId, amount, tx)` — auto-creates user; stamps TransactionId, Type, Amount
- `ShowPointsBalance(userId) (int, error)`
- `GetTransactionHistory(userId) ([]*Transaction, error)`
- `RecordWatchEvent(userId, item)` — updates snapshot, streak, completion counter
- `GetWatchSnapshot(userId) (*UserWatchSnapshot, error)`
- `GetWatchedToday(userId) (int, error)` — filters WatchHistory by today's calendar date
- `GetWatchedYesterday(userId) (int, error)`

### Business Rules in DB
- Content "completed" = `CurrentWatchTime >= 0.9 * TotalRunningTime`
- Streak increments when consecutive calendar days; resets on gap; unchanged on same day
- All users are auto-created on first `CreditPointsBalance` or `RecordWatchEvent` call
- `Transaction.Event` string is used as a semantic label (also used for reward dedup)

---

## Planned: DB Interfaces (`db/interfaces.go`)

```go
type WatchDB interface {
    RecordWatchEvent(userId string, item WatchHistoryLineItem) error
    GetWatchedToday(userId string) (int, error)
    GetWatchedYesterday(userId string) (int, error)
    GetWatchSnapshot(userId string) (*UserWatchSnapshot, error)
}

type RewardsDB interface {
    CreditPointsBalance(userId string, amount int, tx Transaction) error
    ShowPointsBalance(userId string) (int, error)
    GetTransactionHistory(userId string) ([]*Transaction, error)
}
```

`ContentDB` and `UserDB` are intentionally skipped — no content or user registry in the DB.

---

## Planned: Config / Rules Layer (`config/rules.go`)

### Reward Rules

| Rule | Condition | Points | Once-per-day? | EventKey |
|---|---|---|---|---|
| `CurrentDayWatchTimeRewardRule` | `WatchedTodaySec > 600` | 10 | Yes | `"DAILY_WATCH_TIME_REWARD"` |
| `ContentCompletionRewardRule` | `CurrentWatchTime >= 0.9 * TotalRunningTime` (VOD only) | 20 | No | `"CONTENT_COMPLETION_REWARD"` |
| `ConsecutiveWatchStreakRewardRule` | First event today AND streak extended | 10 | Yes | `"STREAK_EXTENSION_REWARD"` |

### Key Design: Once-per-day dedup
Uses `Transaction.Event` as a semantic key. `firedTodayForKey(txHistory, key, now)` scans transaction history for a matching `Event` string on today's date — no extra DB fields needed.

### Interfaces
```go
type RuleContext struct {
    Event           db.WatchHistoryLineItem
    Snapshot        *db.UserWatchSnapshot  // post-recording
    TxHistory       []*db.Transaction      // before this event's credits
    WatchedTodaySec int
    Now             time.Time
}

type RewardRule interface {
    Evaluate(ctx RuleContext) (points int, eventKey string, fire bool)
}
```

---

## Planned: Service Layer (`service/user_rewards_service.go`)

```go
type UserRewardsService struct {
    watchDB   db.WatchDB
    rewardsDB db.RewardsDB
    config    config.RewardConfig
}

func (s *UserRewardsService) Process(userId string, events []db.WatchHistoryLineItem) error
func (s *UserRewardsService) ShowUserCurrentRewardBalance(userId string) (int, error)
func (s *UserRewardsService) TotalWatchHours(userId string) (float64, error)
func (s *UserRewardsService) CurrentStreak(userId string) (int, error)
func (s *UserRewardsService) HighestStreak(userId string) (int, error)
```

### `Process` per-event loop
1. `RecordWatchEvent` → 2. fetch fresh snapshot + today's seconds + tx history → 3. build `RuleContext` → 4. evaluate each rule → 5. `CreditPointsBalance` for each fired rule.

Fetches are **per-event** (not per-batch) so that within-batch credits are visible to `firedTodayForKey` before the next event is evaluated.

---

## Testing Conventions
- Tests live in `db/db_test.go` (same package `db`)
- Run with race detector: `go test -race ./...`
- Each rule should have isolated unit tests in `config/`
- Service tests in `service/` use a real `*DB` (no mocks)

---

## Pending Implementation
The service layer is approved but not yet implemented. Next steps:
1. Add `HighestStreak int` to `UserWatchSnapshot`, update `updateStreak()` in `db/db.go`
2. Create `db/interfaces.go`
3. Create `config/rules.go`
4. Create `service/user_rewards_service.go`
5. Update `main.go` to wire up and use `UserRewardsService`
