# Plan: In-Memory DB Layer for User Rewards + Watch Timeline

## Context
The hotstar project is greenfield — only design comments exist in `main.go`, no executable code yet. The goal is to build an in-memory storage layer with two concerns:
1. **User Rewards** — track points balance and credit transaction history
2. **Watch Timeline** — track per-user watch history, streak, and aggregated watch stats (today / yesterday)

---

## Data Structures

### File: `db/db.go` (new package)

```
// --- Rewards ---

TransactionType  string const  — "CREDIT"

Transaction struct
  TransactionId  string          // auto-incrementing counter
  Type           TransactionType
  Amount         int
  ContentId      string
  Timestamp      time.Time
  Event          string          // "VOD_CONTENT_WATCH_TIME" | "LIVE_STREAM"

UserReward struct
  CurrentPointsBalance int

// --- Watch Timeline ---

ContentType  string const  — "VOD" | "LIVE_STREAM"

WatchHistoryLineItem struct
  ContentId           string
  CurrentWatchTime    int         // seconds watched so far (progress position)
  TotalRunningTime    int         // total content duration in seconds
  ContentType         ContentType
  Timestamp          time.Time   // when the watch event was recorded

UserWatchSnapshot struct
  UserId               string
  StreakCount          int
  TotalContentCompleted int
  TotalWatchedSeconds  int        // all-time cumulative seconds watched across all content
  LastWatchDuration    int        // seconds watched in the last event
  LastSeenTimestamp    time.Time
  WatchHistory         []WatchHistoryLineItem

// --- DB root ---

DB struct
  UserRewards        map[string]*UserReward       // keyed by userId
  TransactionHistory map[string][]*Transaction     // keyed by userId
  UserWatchSnapshots map[string]*UserWatchSnapshot // keyed by userId
  mu                 sync.RWMutex
```

> **Naming note:** Your design called the field `currentWatchTimeStamp` but the struct already has `timestamp`. Based on the existing event shape in `main.go` (`currWatchTime`), this field is a **duration in seconds**, not a timestamp — renamed to `CurrentWatchTime int` above. Confirm this is correct.

---

## Functions

### Rewards
| Function | Notes |
|---|---|
| `CreditPointsBalance(userId, amount, tx)` | Adds to balance, appends tx to history |
| `ShowPointsBalance(userId)` | Returns current points balance |
| `GetTransactionHistory(userId)` | Returns full transaction slice |

### Watch Timeline
| Function | Notes |
|---|---|
| `RecordWatchEvent(userId, item WatchHistoryLineItem)` | Appends to history; increments `TotalWatchedSeconds` by `item.CurrentWatchTime`; increments `TotalContentCompleted` if `CurrentWatchTime >= 0.9 * TotalRunningTime` |
| `GetWatchedToday(userId)` | Filters `WatchHistory` by today's calendar date, sums `CurrentWatchTime` |
| `GetWatchedYesterday(userId)` | Filters `WatchHistory` by yesterday's calendar date, sums `CurrentWatchTime` |
| `GetWatchSnapshot(userId)` | Returns the full UserWatchSnapshot |

### Constructor
| Function | Notes |
|---|---|
| `NewDB() *DB` | Initialises all maps; required before any other call |

---

## Proposed Function Signatures

```go
func NewDB() *DB

// Rewards
func (db *DB) CreditPointsBalance(userId string, amount int, tx Transaction) error
func (db *DB) ShowPointsBalance(userId string) (int, error)
func (db *DB) GetTransactionHistory(userId string) ([]Transaction, error)

// Watch Timeline
func (db *DB) RecordWatchEvent(userId string, item WatchHistoryLineItem) error
func (db *DB) GetWatchedToday(userId string) (int, error)      // returns seconds
func (db *DB) GetWatchedYesterday(userId string) (int, error)  // returns seconds
func (db *DB) GetWatchSnapshot(userId string) (*UserWatchSnapshot, error)
```

---

## Thread Safety
`sync.RWMutex` on the `DB` struct:
- `RLock` for all reads
- `Lock` for all writes

---

## File Layout

```
hotstar/
├── go.mod                   (needs to be created — module name: hotstar)
├── main.go                  (existing — will eventually wire up the DB)
├── docs/plans/PLAN.md       (this file)
└── db/
    └── db.go                (new — all structs + functions above)
```

---

## Open Questions
1. **`currentWatchTimeStamp` naming** — confirmed as `CurrentWatchTime int` (seconds progress), not a timestamp?
2. **Streak definition** — what makes a streak day? Any watch event recorded on that calendar day?
3. ~~**`TotalWatchedSeconds` scope**~~ — resolved: all-time cumulative total at user level
4. ~~**`totalContentCompleted` rule**~~ — resolved: 90% threshold (`CurrentWatchTime >= 0.9 * TotalRunningTime`)
5. **User auto-creation** — should `CreditPointsBalance` / `RecordWatchEvent` auto-create the user if the userId doesn't exist, or require an explicit step first?

---

## Verification
- Unit tests in `db/db_test.go`:
  - Credit then `ShowPointsBalance` returns correct sum
  - `GetTransactionHistory` returns entries in insertion order
  - `RecordWatchEvent` updates `LastSeenTimestamp` and `LastWatchDuration`
  - `GetWatchedToday` sums only events from the current calendar day
  - `GetWatchedYesterday` sums only events from the previous calendar day
  - Concurrent credits/watch events produce correct final state (`go test -race`)
