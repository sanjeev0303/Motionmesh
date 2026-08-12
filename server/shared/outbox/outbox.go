package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motionmesh/server/shared/logger"
	"github.com/nats-io/nats.go"
)

// InsertEvent saves an event into the outbox table using a provided transaction.
func InsertEvent(ctx context.Context, tx pgx.Tx, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (subject, payload)
		VALUES ($1, $2)
	`, subject, data)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// Relay handles polling the outbox table and publishing to NATS.
type Relay struct {
	db  *pgxpool.Pool
	js  nats.JetStreamContext
	log *logger.Logger
}

func NewRelay(db *pgxpool.Pool, nc *nats.Conn, log *logger.Logger) (*Relay, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get jetstream context: %w", err)
	}
	return &Relay{
		db:  db,
		js:  js,
		log: log,
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
	// Simple polling: Get up to 100 unpublished events
	// Uses SKIP LOCKED to allow multiple relayers to run concurrently without duplicate publishing
	
	// Start a transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.log.Error("Failed to begin outbox tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, subject, payload 
		FROM outbox_events 
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 100
	`)
	if err != nil {
		r.log.Error("Failed to query outbox events: %v", err)
		return
	}
	defer rows.Close()

	type event struct {
		id      string
		subject string
		payload []byte
	}

	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.id, &e.subject, &e.payload); err != nil {
			r.log.Error("Failed to scan outbox event: %v", err)
			return
		}
		events = append(events, e)
	}
	
	rows.Close() // close before publishing to not hold the rows longer than necessary, tx is still holding the locks

	if len(events) == 0 {
		return
	}

	for _, e := range events {
		_, err := r.js.Publish(e.subject, e.payload)
		if err != nil {
			r.log.Error("Failed to publish outbox event to NATS (subject: %s): %v", e.subject, err)
			// Break to retry later instead of marking as published
			continue
		}

		// Mark as published
		_, err = tx.Exec(ctx, `UPDATE outbox_events SET published_at = NOW() WHERE id = $1`, e.id)
		if err != nil {
			r.log.Error("Failed to mark outbox event as published: %v", err)
		}
	}
	
	if err := tx.Commit(ctx); err != nil {
		r.log.Error("Failed to commit outbox tx: %v", err)
	}
}
