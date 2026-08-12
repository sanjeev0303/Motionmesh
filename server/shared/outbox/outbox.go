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
	// Claim up to 100 unpublished events by setting claimed_until to 1 minute in the future
	// Uses SKIP LOCKED to allow multiple relayers to run concurrently without duplicate publishing
	
	query := `
		UPDATE outbox_events
		SET claimed_until = NOW() + INTERVAL '1 minute'
		WHERE id IN (
			SELECT id 
			FROM outbox_events 
			WHERE published_at IS NULL 
			  AND (claimed_until IS NULL OR claimed_until < NOW())
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 100
		)
		RETURNING id, subject, payload
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.log.Error("Failed to claim outbox events: %v", err)
		return
	}

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
			rows.Close()
			return
		}
		events = append(events, e)
	}
	rows.Close()

	if len(events) == 0 {
		return
	}

	var publishedIDs []string

	// Send all publishes asynchronously to pipeline the network roundtrips
	futures := make([]nats.PubAckFuture, 0, len(events))
	eventMap := make(map[nats.PubAckFuture]string) // map future to event ID

	for _, e := range events {
		f, err := r.js.PublishAsync(e.subject, e.payload)
		if err != nil {
			r.log.Error("Failed to enqueue async publish for outbox event (subject: %s): %v", e.subject, err)
			continue
		}
		futures = append(futures, f)
		eventMap[f] = e.id
	}

	// Wait for all acks
	for _, f := range futures {
		select {
		case <-f.Ok():
			publishedIDs = append(publishedIDs, eventMap[f])
		case err := <-f.Err():
			r.log.Error("Async publish failed: %v", err)
		case <-ctx.Done():
			r.log.Error("Context cancelled while waiting for publish acks")
			// Stop waiting for the rest if context is done
			goto UpdateDB
		}
	}

UpdateDB:
	if len(publishedIDs) == 0 {
		return
	}

	// Bulk update published items
	_, err = r.db.Exec(ctx, `UPDATE outbox_events SET published_at = NOW() WHERE id = ANY($1)`, publishedIDs)
	if err != nil {
		r.log.Error("Failed to bulk mark outbox events as published: %v", err)
	}
}
