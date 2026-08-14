package auth

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/motionmesh/server/shared/logger"
	"github.com/motionmesh/server/shared/metrics"
	"github.com/redis/go-redis/v9"
)

const (
	lastUsedDebounce  = 30 * time.Second
	lastUsedHashKey   = "mot:api_key:last_used_buffer"
	lastUsedQueueSize = 10000
)

var (
	lastUsedQueue = make(chan string, lastUsedQueueSize)
	workerOnce    sync.Once
)

func startLastUsedWorker(rdb *redis.Client) {
	workerOnce.Do(func() {
		go func() {
			ctx := context.Background()
			for keyID := range lastUsedQueue {
				start := time.Now()
				lockKey := fmt.Sprintf("mot:api_key:last_used_lock:%s", keyID)
				ok, err := rdb.SetNX(ctx, lockKey, "1", lastUsedDebounce).Result()
				if err == nil && ok {
					rdb.HSet(ctx, lastUsedHashKey, keyID, time.Now().Unix())
				}
				metrics.LastUsedWorkerLatency.Observe(time.Since(start).Seconds())
			}
		}()
	})
}

// trackLastUsed attempts to enqueue the keyID for last-used tracking.
// It relies on a bounded buffered queue to prevent goroutine exhaustion
// on the hot path.
func trackLastUsed(rdb *redis.Client, keyID string) {
	startLastUsedWorker(rdb)
	select {
	case lastUsedQueue <- keyID:
		metrics.LastUsedEnqueueTotal.Inc()
	default:
		metrics.LastUsedDroppedTotal.Inc()
	}
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
