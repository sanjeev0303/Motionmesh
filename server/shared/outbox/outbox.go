package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motionmesh/server/shared/logger"
	"github.com/nats-io/nats.go"
)

// InsertEvent saves an event into the outbox table using a provided transaction.
func InsertEvent(ctx context.Context, tx pgx.Tx, eventID string, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, subject, payload, status)
		VALUES ($1, $2, $3, 'pending')
	`, eventID, subject, data)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// Relay handles polling the outbox table and publishing to NATS.
type Relay struct {
	db                 *pgxpool.Pool
	js                 nats.JetStreamContext
	log                *logger.Logger
	batchSize          int
	maxAttempts        int
	publishConcurrency int
}

func NewRelay(db *pgxpool.Pool, nc *nats.Conn, batchSize, maxAttempts int, log *logger.Logger) (*Relay, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get jetstream context: %w", err)
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	
	concurrency := 50
	if val := os.Getenv("OUTBOX_PUBLISH_CONCURRENCY"); val != "" {
		if c, err := strconv.Atoi(val); err == nil && c > 0 {
			concurrency = c
		}
	}
	
	return &Relay{
		db:                 db,
		js:                 js,
		log:                log,
		batchSize:          batchSize,
		maxAttempts:        maxAttempts,
		publishConcurrency: concurrency,
	}, nil
}

func (r *Relay) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("Outbox relay shutting down")
			return
		case <-ticker.C:
			r.processOutbox(ctx)
		}
	}
}

func (r *Relay) processOutbox(ctx context.Context) {
	query := fmt.Sprintf(`
		UPDATE outbox_events
		SET claimed_until = NOW() + INTERVAL '1 minute', status = 'publishing'
		WHERE id IN (
			SELECT id
			FROM outbox_events
			WHERE status IN ('pending', 'failed')
			  AND (claimed_until IS NULL OR claimed_until < NOW())
			  AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
			  AND attempts < %d
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT %d
		)
		RETURNING id, subject, payload, attempts
	`, r.maxAttempts, r.batchSize)

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.log.Error("Failed to claim outbox events: %v", err)
		return
	}

	type event struct {
		id       string
		subject  string
		payload  []byte
		attempts int
	}

	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.id, &e.subject, &e.payload, &e.attempts); err != nil {
			r.log.Error("Failed to scan outbox event: %v", err)
			rows.Close()
			return
		}
		events = append(events, e)
	}
	rows.Close()

	if len(events) == 0 {
		return
	}

	var (
		mu           sync.Mutex
		publishedIDs []string
		failedEvents []event
		errorMap     = make(map[string]string)
	)

	// Bounded concurrency pool for publishing
	sem := make(chan struct{}, r.publishConcurrency)
	var wg sync.WaitGroup

	for _, e := range events {
		wg.Add(1)
		sem <- struct{}{} // acquire
		
		go func(ev event) {
			defer wg.Done()
			defer func() { <-sem }() // release

			// We could use PublishAsync, but with a bounded pool we can just use synchronous Publish
			// combined with an overall context timeout if desired, or rely on NATS core timeout.
			// The prompt allows Async but bounded in-flight is requested. Let's use Sync Publish 
			// inside goroutines which naturally bounds in-flight publishes.
			
			// Publish timeout can be passed via context or implicit in NATS.
			// nats.MaxWait is typically for subscribers, for publishers it's bound by connection.
			_, err := r.js.Publish(ev.subject, ev.payload)
			
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				r.log.Error("Failed to publish outbox event (id: %s, subject: %s): %v", ev.id, ev.subject, err)
				failedEvents = append(failedEvents, ev)
				errorMap[ev.id] = err.Error()
			} else {
				publishedIDs = append(publishedIDs, ev.id)
			}
		}(e)
	}
	
	wg.Wait()

	// 1. Mark published
	if len(publishedIDs) > 0 {
		_, err = r.db.Exec(ctx,
			`UPDATE outbox_events SET status = 'published', published_at = NOW() WHERE id = ANY($1::text[])`,
			publishedIDs,
		)
		if err != nil {
			r.log.Error("Failed to bulk mark outbox events as published: %v", err)
		}
	}

	// 2. Handle failures with exponential backoff & dead letter
	if len(failedEvents) > 0 {
		for _, ev := range failedEvents {
			newAttempts := ev.attempts + 1
			status := "failed"
			if newAttempts >= r.maxAttempts {
				status = "dead_letter"
			}
			
			// Exponential backoff: 2^attempts seconds
			backoffSeconds := 1 << ev.attempts
			
			_, ferr := r.db.Exec(ctx,
				`UPDATE outbox_events 
				 SET attempts = $1, status = $2, next_attempt_at = NOW() + INTERVAL '1 second' * $3, last_error = $4
				 WHERE id = $5`,
				newAttempts, status, backoffSeconds, errorMap[ev.id], ev.id,
			)
			if ferr != nil {
				r.log.Error("Failed to update failed outbox event %s: %v", ev.id, ferr)
			}
		}
	}
}
