# File Locking for Agent Session Registry

## Overview

Warren's agent session registry uses file locking to prevent corruption when multiple Warren instances access the same registry file concurrently. This is critical for distributed deployments where multiple Warren processes might run simultaneously.

## Implementation

### Technology

- **Library**: `github.com/gofrs/flock` v0.13.0
- **Platform Support**: Cross-platform (Linux, macOS, Windows)
- **Lock Type**: Advisory file locks (flock on Unix, LockFileEx on Windows)

### Lock Mechanism

The registry uses a separate lock file (`registry.json.lock`) to coordinate access:

```go
lockPath := path + ".lock"
fileLock := flock.New(lockPath)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
if err != nil {
    return fmt.Errorf("failed to acquire lock: %w", err)
}
if !locked {
    return fmt.Errorf("failed to acquire lock: timeout after 5 seconds")
}
defer fileLock.Unlock()
```

### Lock Behavior

1. **Timeout**: 5 seconds maximum wait time
2. **Retry Interval**: 100ms between lock attempts
3. **Scope**: Locks are held only during file I/O operations
4. **Cleanup**: Locks are automatically released via `defer`

## Usage

### Save Operation

```go
registry := core.NewAgentSessionRegistry()
// ... add sessions ...

// Save with automatic locking
if err := registry.Save("/path/to/registry.json"); err != nil {
    log.Fatalf("Failed to save: %v", err)
}
```

The `Save()` method:
1. Acquires exclusive lock on `registry.json.lock`
2. Writes data to temporary file (`registry.json.tmp`)
3. Atomically renames temp file to `registry.json`
4. Releases lock

### Load Operation

```go
registry := core.NewAgentSessionRegistry()

// Load with automatic locking
if err := registry.Load("/path/to/registry.json"); err != nil {
    log.Fatalf("Failed to load: %v", err)
}
```

The `Load()` method:
1. Acquires shared lock on `registry.json.lock`
2. Reads `registry.json`
3. Parses JSON data
4. Releases lock

## Concurrency Guarantees

### What File Locking Prevents

✅ **Prevents**:
- Corrupted JSON from simultaneous writes
- Partial reads during writes
- Race conditions between multiple Warren instances

### What File Locking Does NOT Prevent

❌ **Does NOT prevent**:
- Lost updates in load-modify-save pattern (last writer wins)
- Stale reads (data may change after lock is released)
- Deadlocks (5-second timeout prevents indefinite blocking)

## Error Handling

### Lock Acquisition Failure

If a lock cannot be acquired within 5 seconds:

```
failed to acquire lock: timeout after 5 seconds
```

**Possible causes**:
- Another Warren instance is holding the lock
- File system doesn't support locking (rare)
- Lock file permissions issue

**Resolution**:
- Wait and retry
- Check for hung Warren processes
- Verify file system supports advisory locks

### Lock File Cleanup

Lock files (`.lock`) are automatically cleaned up when:
- The process releases the lock normally
- The process terminates (OS cleans up advisory locks)

**Note**: Lock files may persist on disk but become invalid when the process exits.

## Performance Characteristics

### Overhead

- **Lock acquisition**: ~1-2ms on local filesystem
- **Lock release**: <1ms
- **Total overhead**: ~2-3ms per Save/Load operation

### Scalability

Tested with:
- ✅ 10 concurrent writers
- ✅ 20 concurrent readers
- ✅ Mixed read/write workloads
- ✅ 2-second stress test with 30 goroutines

See `internal/core/agent_session_persistence_test.go` for test details.

## Testing

### Unit Tests

Run concurrent access tests:

```bash
go test -v ./internal/core -run TestAgentSessionRegistry_Concurrent
```

Tests include:
- `TestAgentSessionRegistry_ConcurrentSave` - Multiple simultaneous saves
- `TestAgentSessionRegistry_ConcurrentLoad` - Multiple simultaneous loads
- `TestAgentSessionRegistry_ConcurrentSaveAndLoad` - Mixed operations
- `TestAgentSessionRegistry_LockTimeout` - Timeout behavior
- `TestAgentSessionRegistry_NoCorruptionUnderLoad` - Stress test

### Demo Program

Run the concurrent access demo:

```bash
go run examples/concurrent_registry_demo.go
```

This simulates 5 Warren instances performing 10 operations each concurrently.

## Best Practices

### For Warren Developers

1. **Always use Save/Load methods** - Don't bypass the registry API
2. **Keep lock duration short** - Don't hold locks during long operations
3. **Handle lock failures gracefully** - Retry or fail fast
4. **Test concurrent scenarios** - Use the provided test suite

### For Warren Operators

1. **Use shared filesystem** - NFS, EFS, or similar for multi-host deployments
2. **Monitor lock contention** - High contention indicates too many instances
3. **Check lock file permissions** - Ensure Warren can create `.lock` files
4. **Verify filesystem support** - Advisory locks must be supported

## Limitations

### Load-Modify-Save Pattern

File locking does NOT solve the lost update problem:

```go
// Instance A
registry.Load(path)           // Loads state with 10 sessions
registry.Register(sessionA)   // Now has 11 sessions
registry.Save(path)           // Saves 11 sessions

// Instance B (concurrent)
registry.Load(path)           // Loads state with 10 sessions
registry.Register(sessionB)   // Now has 11 sessions
registry.Save(path)           // Saves 11 sessions (sessionA is lost!)
```

**Solution**: Warren's discovery mechanism re-discovers sessions from tmux, so lost updates are recovered on the next scan.

### Network Filesystems

Advisory locks on NFS require:
- NFSv4 or later
- Proper lock daemon configuration (`lockd`, `statd`)
- Client-side lock support enabled

Test lock behavior on your specific NFS setup before deploying.

## Future Enhancements

Potential improvements (not currently implemented):

1. **Optimistic locking** - Version numbers to detect conflicts
2. **Lock-free data structures** - Append-only event log
3. **Distributed locking** - etcd/Consul for multi-host coordination
4. **Lock metrics** - Prometheus metrics for lock contention

## References

- [github.com/gofrs/flock](https://github.com/gofrs/flock) - File locking library
- [flock(2)](https://man7.org/linux/man-pages/man2/flock.2.html) - Linux flock system call
- [LockFileEx](https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex) - Windows file locking
