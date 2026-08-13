package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/motionmesh/server/shared/logger"
	"github.com/motionmesh/server/shared/models"
	"github.com/nats-io/nats.go"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billing/meterevent"
	"golang.org/x/sync/singleflight"
)

type Repository interface {
	GetAccountByID(ctx context.Context, id string) (*models.Account, error)
	RecordUsageEvent(ctx context.Context, event *models.UsageEvent) error
}

type cachedAccount struct {
	acc       *models.Account
	expiresAt time.Time
}

type Consumer struct {
	repo            Repository
	stripeSecretKey string
	log             *logger.Logger
	
	// Stripe bounded worker pool
	stripeJobs chan stripeJob
	wg         sync.WaitGroup

	// Account cache
	accCache map[string]cachedAccount
	accMu    sync.RWMutex
	accSF    singleflight.Group
}

type stripeJob struct {
	AccountID        string
	StripeCustomerID string
	EventType        string
	Quantity         int64
	EventID          string
	Attempt          int
}

func NewConsumer(repo Repository, stripeSecretKey string, log *logger.Logger) *Consumer {
	stripe.Key = stripeSecretKey
	
	c := &Consumer{
		repo:            repo,
		stripeSecretKey: stripeSecretKey,
		log:             log,
		stripeJobs:      make(chan stripeJob, 1000),
		accCache:        make(map[string]cachedAccount),
	}
	
	concurrency := 10
	if val := os.Getenv("STRIPE_WORKER_CONCURRENCY"); val != "" {
		if c, err := strconv.Atoi(val); err == nil && c > 0 {
			concurrency = c
		}
	}
	
	c.log.Info("Starting %d Stripe bounded workers", concurrency)
	for i := 0; i < concurrency; i++ {
		c.wg.Add(1)
		go c.stripeWorker(i)
	}
	
	return c
}

func (c *Consumer) Stop() {
	close(c.stripeJobs)
	c.wg.Wait()
}

func (c *Consumer) getAccountCached(ctx context.Context, accountID string) (*models.Account, error) {
	c.accMu.RLock()
	cached, ok := c.accCache[accountID]
	c.accMu.RUnlock()
	
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.acc, nil
	}
	
	v, err, _ := c.accSF.Do(accountID, func() (interface{}, error) {
		acc, err := c.repo.GetAccountByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if acc == nil {
			return nil, nil
		}
		
		c.accMu.Lock()
		c.accCache[accountID] = cachedAccount{
			acc:       acc,
			expiresAt: time.Now().Add(5 * time.Minute),
		}
		c.accMu.Unlock()
		
		return acc, nil
	})
	
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(*models.Account), nil
}

func (c *Consumer) stripeWorker(id int) {
	defer c.wg.Done()
	
	for job := range c.stripeJobs {
		// Respect mock mode for load testing
		if os.Getenv("STRIPE_MOCK_MODE") == "true" {
			continue // simulate success
		}

		params := &stripe.BillingMeterEventParams{
			EventName: stripe.String(job.EventType),
			Payload: map[string]string{
				"stripe_customer_id": job.StripeCustomerID,
				"value":              fmt.Sprintf("%d", job.Quantity),
			},
		}
		
		// Optional idempotency key (if Stripe SDK allows it on MeterEvent)
		// params.IdempotencyKey = stripe.String(job.EventID)

		_, err := meterevent.New(params)
		if err != nil {
			c.log.Error("stripe worker %d: failed to report meter event %s for %s (attempt %d): %v", id, job.EventType, job.AccountID, job.Attempt, err)
			
			// Simple backoff retry (up to 3 attempts in memory)
			if job.Attempt < 3 {
				job.Attempt++
				go func(j stripeJob) {
					time.Sleep(time.Duration(j.Attempt*2) * time.Second)
					c.stripeJobs <- j
				}(job)
			} else {
				// Record dead-letter / failure state (could insert into a DB table)
				c.log.Error("stripe worker %d: permanently failed meter event %s for %s", id, job.EventType, job.AccountID)
			}
		}
	}
}

// ReportUsage writes to usage_events (source of truth) and queues a Stripe Meter Event.
func (c *Consumer) ReportUsage(ctx context.Context, eventID, accountID, eventType string, qty int64, stripeCustomerID string) error {
	event := &models.UsageEvent{
		ID:        eventID,
		EventID:   eventID,
		AccountID: accountID,
		EventType: eventType,
		Quantity:  qty,
		CreatedAt: time.Now(),
	}
	if err := c.repo.RecordUsageEvent(ctx, event); err != nil {
		return fmt.Errorf("billing: record usage event: %w", err)
	}

	// Queue Stripe API call so it doesn't block the hot path
	c.stripeJobs <- stripeJob{
		AccountID:        accountID,
		StripeCustomerID: stripeCustomerID,
		EventType:        eventType,
		Quantity:         qty,
		EventID:          eventID,
		Attempt:          1,
	}
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

			// Optionally could process these in parallel, but DB insertion is usually fast.
			for _, msg := range msgs {
				var ev usageEvent
				if err := json.Unmarshal(msg.Data, &ev); err != nil {
					c.log.Error("unmarshal usage event: %v", err)
					msg.Term()
					continue
				}

				acc, err := c.getAccountCached(ctx, ev.AccountID)
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

				meta, metaErr := msg.Metadata()
				var eventID string
				if metaErr == nil {
					eventID = fmt.Sprintf("nats_%s_%d", meta.Stream, meta.Sequence.Stream)
				} else {
					eventID = uuid.NewMD5(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s_%d", ev.VideoID, time.Now().Unix()/300))).String()
				}

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
