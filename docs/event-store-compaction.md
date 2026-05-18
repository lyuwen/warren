# Event Store Compaction

## Overview

Warren's event store automatically prunes old events to prevent unbounded database growth. This document describes the compaction mechanism and how to configure it.

## How It Works

### Automatic Pruning

When Warren starts, it automatically launches a background goroutine that periodically deletes events older than the configured retention period. This happens transparently without user intervention.

**Default Configuration:**
- **Retention Period:** 30 days
- **Pruning Interval:** 24 hours (daily)

### What Gets Pruned

All event types are subject to pruning based on their `timestamp` field:
- Activity events (`EventTypeActivity`)
- Notification events (`EventTypeNotification`)
- State change events (`EventTypeStateChange`)

Events are pruned based on their timestamp, not when they were created. This means an event with a timestamp 31 days in the past will be pruned even if it was just inserted.

### Pruning Process

1. **Initial Run:** Pruning runs immediately when the event store starts
2. **Periodic Runs:** Pruning runs at the configured interval (default: daily)
3. **Logging:** Each pruning operation logs:
   - Number of events deleted
   - Time taken to complete
   - Only logs if events were actually deleted (silent if nothing to prune)

Example log output:
```
[EventStore] Pruned 1523 events older than 720h0m0s (took 45.2ms)
```

## Configuration

### Warren Configuration

Configure retention and pruning via the `Config` struct:

```go
config := &core.Config{
    DBPath:               "warren.db",
    EventRetentionPeriod: 30 * 24 * time.Hour, // 30 days
    EventPruningInterval: 24 * time.Hour,      // daily
}

warren, err := core.NewWarren(config)
```

### Event Store Configuration

You can also configure the event store directly:

```go
storeConfig := &events.StoreConfig{
    DBPath:          "warren.db",
    RetentionPeriod: 7 * 24 * time.Hour,  // 7 days
    PruningInterval: 12 * time.Hour,      // twice daily
}

store, err := events.NewStoreWithConfig(storeConfig)
if err != nil {
    log.Fatal(err)
}

// Start background pruning
store.StartPruningJob()
```

### Configuration Validation

Invalid configuration values are automatically corrected:
- **Zero or negative retention period** → defaults to 30 days
- **Zero or negative pruning interval** → defaults to 24 hours

This prevents panics from invalid ticker intervals.

## Manual Pruning

You can manually trigger pruning without waiting for the background job:

```go
// Prune events older than 30 days
deleted, err := store.PruneOldEvents(30 * 24 * time.Hour)
if err != nil {
    log.Printf("Pruning failed: %v", err)
} else {
    log.Printf("Pruned %d events", deleted)
}
```

## Recommended Configurations

### Development/Testing
```go
EventRetentionPeriod: 7 * 24 * time.Hour,  // 7 days
EventPruningInterval: 1 * time.Hour,       // hourly
```

### Production (Default)
```go
EventRetentionPeriod: 30 * 24 * time.Hour, // 30 days
EventPruningInterval: 24 * time.Hour,      // daily
```

### Long-Term Monitoring
```go
EventRetentionPeriod: 90 * 24 * time.Hour, // 90 days
EventPruningInterval: 24 * time.Hour,      // daily
```

### High-Volume Environments
```go
EventRetentionPeriod: 14 * 24 * time.Hour, // 14 days
EventPruningInterval: 12 * time.Hour,      // twice daily
```

## Performance Considerations

### Database Size

Without pruning, the event store grows indefinitely:
- **Typical event size:** ~500 bytes
- **High-activity agent:** ~1000 events/day
- **10 agents, 30 days:** ~150 MB

With pruning enabled (30-day retention):
- Database size stabilizes at ~150 MB
- Pruning operation takes <100ms for typical workloads

### Pruning Performance

Pruning is efficient due to:
- Indexed `timestamp` column
- Single DELETE query with WHERE clause
- Runs in background, doesn't block operations

**Typical Performance:**
- 1,000 events: <10ms
- 10,000 events: <50ms
- 100,000 events: <200ms

### Recommendations

1. **Don't prune too frequently:** Hourly is usually overkill, daily is sufficient
2. **Balance retention vs. disk space:** 30 days is a good default
3. **Monitor database size:** Use `SELECT COUNT(*) FROM events` to track growth
4. **Consider VACUUM:** SQLite benefits from periodic VACUUM operations

## Lifecycle Management

### Startup

```go
warren, err := core.NewWarren(config)
// Pruning job starts automatically
```

### Shutdown

```go
warren.Stop()
// Pruning job stops automatically when event store closes
```

The pruning goroutine is properly cleaned up when the store closes, preventing goroutine leaks.

## Troubleshooting

### Database Growing Too Large

**Symptom:** Database file size keeps growing despite pruning

**Solutions:**
1. Check retention period: `SELECT MIN(timestamp) FROM events`
2. Verify pruning is running: Look for `[EventStore] Pruned` log messages
3. Run manual VACUUM: `sqlite3 warren.db "VACUUM;"`

### Pruning Not Running

**Symptom:** No pruning log messages

**Possible Causes:**
1. No old events to prune (working as intended)
2. Pruning interval too long (check config)
3. Event store not started properly

**Debug:**
```go
// Check event count
count, _ := store.Count()
log.Printf("Total events: %d", count)

// Check oldest event
events, _ := store.Query(events.QueryOptions{
    Limit: 1,
    // Oldest first
})
```

### Performance Issues

**Symptom:** Pruning takes too long

**Solutions:**
1. Verify `timestamp` index exists: `CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp)`
2. Reduce retention period to prune fewer events per run
3. Increase pruning interval to run less frequently
4. Consider archiving old events instead of deleting

## Testing

The pruning functionality is thoroughly tested:

```bash
# Run pruning tests
go test -v ./internal/events -run TestPrune

# Check coverage
go test -coverprofile=coverage.out ./internal/events
go tool cover -func=coverage.out | grep Prune
```

**Test Coverage:**
- Basic pruning with old and new events
- Empty database (no events to prune)
- No old events (all events recent)
- Configurable retention periods
- Multiple event types
- Background job execution
- Graceful shutdown

## Future Enhancements

Potential improvements for future versions:

1. **Archiving:** Export old events to archive files before deletion
2. **Metrics:** Expose pruning metrics (events deleted, time taken, errors)
3. **Selective Pruning:** Different retention periods per event type
4. **Compression:** Compress old events instead of deleting
5. **Manual Control:** API endpoint to trigger pruning on demand

## See Also

- [Event Store Design](../design-review.md#event-store)
- [Database Schema](../internal/events/store.go)
- [Warren Configuration](../CLAUDE.md#configuration)
