package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/motionmesh/server/shared/logger"
	"github.com/redis/go-redis/v9"
)

const (
	lastUsedDebounce = 30 * time.Second
	lastUsedHashKey  = "mot:api_key:last_used_buffer"
)

// trackLastUsed fires a background goroutine that records the current time in a
// Redis buffer hash for the given keyID, gated by a per-key lock so we write
// at most once per debounce window. The goroutine is intentionally detached from
// the request context so it doesn't cancel mid-flight.
func trackLastUsed(rdb *redis.Client, keyID string) {
	go func() {
		ctx := context.Background()
		lockKey := fmt.Sprintf("mot:api_key:last_used_lock:%s", keyID)
		ok, err := rdb.SetNX(ctx, lockKey, "1", lastUsedDebounce).Result()
		if err != nil || !ok {
			// Already tracked within this window, or Redis is temporarily down.
			// Neither case warrants a log entry — this is the hot auth path.
			return
		}
		rdb.HSet(ctx, lastUsedHashKey, keyID, time.Now().Unix())
	}()
}

// FlushLastUsedLoop is started once from main.go.
// It wakes up on every interval, drains the Redis buffer, writes to Postgres in bulk,
// and only clears the buffer on success — failed flushes are retried next tick.
func FlushLastUsedLoop(ctx context.Context, rdb *redis.Client, repo AccountRepository, interval time.Duration) {
	log := logger.New()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			entries, err := rdb.HGetAll(ctx, lastUsedHashKey).Result()
			if err != nil {
				log.Error("last-used flush: HGetAll: %v", err)
				continue
			}
			if len(entries) == 0 {
				continue
			}

			// Convert string unix timestamps to time.Time for the batch writer.
			updates := make(map[string]time.Time, len(entries))
			for keyID, tsStr := range entries {
				ts, err := strconv.ParseInt(tsStr, 10, 64)
				if err != nil {
					log.Error("last-used flush: parse ts for key %s: %v", keyID, err)
					continue
				}
				updates[keyID] = time.Unix(ts, 0)
			}

			if err := repo.BatchUpdateLastUsed(ctx, updates); err != nil {
				log.Error("last-used flush: batch update failed: %v — buffer preserved for next tick", err)
				continue // leave the buffer intact; data is not lost
			}

			// Only clear once the DB write succeeds.
			if err := rdb.Del(ctx, lastUsedHashKey).Err(); err != nil {
				log.Error("last-used flush: del buffer: %v", err)
			}
		}
	}
}
