package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/motionmesh/server/shared/logger"
	"github.com/motionmesh/server/shared/models"
	"github.com/nats-io/nats.go"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billing/meterevent"
)

type Repository interface {
	GetAccountByID(ctx context.Context, id string) (*models.Account, error)
	RecordUsageEvent(ctx context.Context, event *models.UsageEvent) error
}

type Consumer struct {
	repo            Repository
	stripeSecretKey string
	log             *logger.Logger
}

func NewConsumer(repo Repository, stripeSecretKey string, log *logger.Logger) *Consumer {
	stripe.Key = stripeSecretKey
	return &Consumer{
		repo:            repo,
		stripeSecretKey: stripeSecretKey,
		log:             log,
	}
}

// ReportUsage writes to usage_events (source of truth) and sends a Stripe Meter Event.
func (c *Consumer) ReportUsage(ctx context.Context, eventID, accountID, eventType string, qty int64, stripeCustomerID string) error {
	event := &models.UsageEvent{
		ID:        eventID,
		AccountID: accountID,
		EventType: eventType,
		Quantity:  qty,
		CreatedAt: time.Now(),
	}
	if err := c.repo.RecordUsageEvent(ctx, event); err != nil {
		return fmt.Errorf("billing: record usage event: %w", err)
	}

	// Make Stripe API calls asynchronous so they don't block the hot path
	go func() {
		// Respect mock mode for load testing
		if os.Getenv("STRIPE_MOCK_MODE") == "true" {
			return
		}

		// Report to Stripe Meters API (not the legacy usage-records API).
		params := &stripe.BillingMeterEventParams{
			EventName: stripe.String(eventType),
			Payload: map[string]string{
				"stripe_customer_id": stripeCustomerID,
				"value":              fmt.Sprintf("%d", qty),
			},
		}
		_, err := meterevent.New(params)
		if err != nil {
			c.log.Error("failed to report stripe meter event for %s: %v", accountID, err)
		}
	}()
	return nil
}

type usageEvent struct {
	AccountID string  `json:"account_id"`
	VideoID   string  `json:"video_id"`
	Duration  float64 `json:"duration"`
}

func (c *Consumer) ConsumeUsageEvents(ctx context.Context, nc *nats.Conn) error {
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get jetstream context: %w", err)
	}

	sub, err := js.PullSubscribe("motionmesh.billing.usage", "billing_usage_worker")
	if err != nil {
		return fmt.Errorf("failed to pull subscribe to usage events: %w", err)
	}

	c.log.Info("Started JetStream consuming usage events on motionmesh.billing.usage")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil {
				if err != nats.ErrTimeout {
					c.log.Error("nats fetch error: %v", err)
				}
				continue
			}

			for _, msg := range msgs {
				var ev usageEvent
				if err := json.Unmarshal(msg.Data, &ev); err != nil {
					c.log.Error("unmarshal usage event: %v", err)
					msg.Term()
					continue
				}

				acc, err := c.repo.GetAccountByID(ctx, ev.AccountID)
				if err != nil || acc == nil {
					c.log.Error("failed to get account %s for usage reporting: %v", ev.AccountID, err)
					msg.Nak()
					continue
				}

				qty := int64(ev.Duration)
				if qty <= 0 {
					qty = 1 // Minimum 1 second billing
				}

				var stripeID string
				if acc.StripeCustomerID != nil {
					stripeID = *acc.StripeCustomerID
				}

				// Deterministic ID for idempotency: NATS event might be redelivered.
				eventID := uuid.NewMD5(uuid.NameSpaceOID, []byte(ev.VideoID+"_transcode")).String()

				err = c.ReportUsage(ctx, eventID, ev.AccountID, "video_transcode_seconds", qty, stripeID)
				if err != nil {
					c.log.Error("failed to report usage for account %s: %v", ev.AccountID, err)
					msg.Nak()
				} else {
					c.log.Info("Reported usage for account %s: %d seconds", ev.AccountID, qty)
					msg.Ack()
				}
			}
		}
	}
}
