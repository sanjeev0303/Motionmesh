package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/motionmesh/server/shared/logger"
	"github.com/motionmesh/server/shared/models"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billing/meter"
	"github.com/stripe/stripe-go/v82/billing/meterevent"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/invoice"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Service handles Stripe Meters API reporting and webhook processing.
// It is the only layer that imports Stripe SDK types.
type Service struct {
	repo           BillingRepository
	rdb            *redis.Client
	webhookSecret  string
	meterEventName string // Stripe Meter name (e.g. "api_requests")
}

func NewService(repo BillingRepository, rdb *redis.Client, stripeSecretKey, webhookSecret string) *Service {
	stripe.Key = stripeSecretKey
	return &Service{
		repo:          repo,
		rdb:           rdb,
		webhookSecret: webhookSecret,
	}
}

// ReportUsage writes to usage_events (source of truth) and sends a Stripe Meter Event.
// Stripe Meter Events are downstream projections — the DB record is authoritative.
func (s *Service) ReportUsage(ctx context.Context, accountID, eventType string, qty int64, stripeCustomerID string) error {
	event := &models.UsageEvent{
		AccountID: accountID,
		EventType: eventType,
		Quantity:  qty,
		CreatedAt: time.Now(),
	}
	if err := s.repo.RecordUsageEvent(ctx, event); err != nil {
		return fmt.Errorf("billing: record usage event: %w", err)
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
	return err
}

// CheckBalance verifies the account hasn't exhausted its plan limits in Postgres.
// Stripe does NOT gate usage — this check must live in our request path.
// It caches the aggregated usage in Redis for a short TTL (30-60s).
func (s *Service) CheckBalance(ctx context.Context, accountID, resourceType string, planLimits map[string]int64) error {
	limit, ok := planLimits[resourceType]
	if !ok {
		return nil
	}

	cacheKey := fmt.Sprintf("usage:%s:%s", accountID, resourceType)
	cached, err := s.rdb.Get(ctx, cacheKey).Int64()
	if err == nil {
		if cached >= limit {
			return errors.New("billing: plan limit reached for " + resourceType)
		}
		return nil
	}

	used, err := s.repo.GetAggregatedUsage(ctx, accountID, resourceType)
	if err != nil {
		return err
	}
	
	// Cache for 60 seconds
	s.rdb.Set(ctx, cacheKey, used, 60*time.Second)

	if used >= limit {
		return errors.New("billing: plan limit reached for " + resourceType)
	}
	return nil
}

// GetAccountPlan fetches the plan for an account, preferring the Redis cache.
func (s *Service) GetAccountPlan(ctx context.Context, accountID string) (string, error) {
	cacheKey := fmt.Sprintf("plan:%s", accountID)
	
	// Try cache first
	plan, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		return plan, nil
	}
	
	// If not in cache, we need to get it from DB. 
	// But billing repo only has GetAccountByStripeCustomerID. We might need a method GetAccountByID.
	// Wait, we can get it from another repo, or add GetAccountByID to BillingRepository.
	// Let's add it to BillingRepository.
	acc, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return "", err
	}
	
	// Cache for 60 seconds
	s.rdb.Set(ctx, cacheKey, acc.Plan, 60*time.Second)
	return acc.Plan, nil
}

// HandleWebhook processes Stripe webhooks. Updates accounts.plan/status in Postgres.
// This is what the client sidebar reads for real-time plan status.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	event, err := webhook.ConstructEvent(payload, sigHeader, s.webhookSecret)
	if err != nil {
		return fmt.Errorf("billing: webhook signature invalid: %w", err)
	}

	switch event.Type {
	case "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		plan := "free"
		status := "active"
		if sub.Status == stripe.SubscriptionStatusActive {
			// Determine plan from the price metadata or product name.
			if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
				plan = sub.Items.Data[0].Price.Nickname
			}
		}
		if sub.Status == stripe.SubscriptionStatusPastDue || sub.Status == stripe.SubscriptionStatusUnpaid {
			status = "suspended"
		}
		acc, err := s.repo.GetAccountByStripeCustomerID(ctx, sub.Customer.ID)
		if err != nil || acc == nil {
			return err
		}
		
		err = s.repo.UpdatePlan(ctx, acc.ID, plan, status)
		if err == nil {
			// Invalidate plan cache
			s.rdb.Del(ctx, fmt.Sprintf("plan:%s", acc.ID))
		}
		return err

	case "invoice.paid":
		// Subscription renewed — ensure status is active.
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return err
		}
		acc, err := s.repo.GetAccountByStripeCustomerID(ctx, inv.Customer.ID)
		if err != nil || acc == nil {
			return err
		}
		
		err = s.repo.UpdatePlan(ctx, acc.ID, acc.Plan, "active")
		if err == nil {
			// Invalidate plan cache
			s.rdb.Del(ctx, fmt.Sprintf("plan:%s", acc.ID))
		}
		return err
	}
	return nil
}

// ListMeters returns all Stripe Meters (for admin/debug purposes).
func (s *Service) ListMeters() ([]*stripe.BillingMeter, error) {
	var meters []*stripe.BillingMeter
	iter := meter.List(&stripe.BillingMeterListParams{})
	for iter.Next() {
		meters = append(meters, iter.BillingMeter())
	}
	return meters, iter.Err()
}

// ListInvoices returns the recent invoices for a Stripe customer.
func (s *Service) ListInvoices(ctx context.Context, stripeCustomerID string) ([]map[string]interface{}, error) {
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(stripeCustomerID),
	}
	params.Filters.AddFilter("limit", "", "10")

	iter := invoice.List(params)
	var invoices []map[string]interface{}
	for iter.Next() {
		inv := iter.Invoice()
		invoices = append(invoices, map[string]interface{}{
			"id":     inv.ID,
			"date":   time.Unix(inv.Created, 0).Format(time.RFC3339),
			"amount": float64(inv.Total) / 100.0,
			"status": inv.Status,
		})
	}
	return invoices, iter.Err()
}

// ParseBody is a helper for reading raw webhook body without consuming it.
func ParseBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

// GetAggregatedUsage returns the usage for a specific event type, utilizing Redis cache when possible.
func (s *Service) GetAggregatedUsage(ctx context.Context, accountID, eventType string) (int64, error) {
	cacheKey := fmt.Sprintf("usage:%s:%s", accountID, eventType)
	cached, err := s.rdb.Get(ctx, cacheKey).Int64()
	if err == nil {
		return cached, nil
	}

	used, err := s.repo.GetAggregatedUsage(ctx, accountID, eventType)
	if err != nil {
		return 0, err
	}

	// Cache for 60 seconds
	s.rdb.Set(ctx, cacheKey, used, 60*time.Second)
	return used, nil
}

// AddFunds adds the specified amount (in cents) to the account's prepaid balance.
func (s *Service) AddFunds(ctx context.Context, accountID string, amount int64) (int64, error) {
	if amount <= 0 {
		return 0, errors.New("billing: amount must be greater than zero")
	}
	return s.repo.AddFunds(ctx, accountID, amount)
}

// CreatePortalSession creates a Stripe Customer Portal session for subscription management.
func (s *Service) CreatePortalSession(ctx context.Context, account *models.Account, returnURL string) (string, error) {
	var customerID string
	if account.StripeCustomerID != nil {
		customerID = *account.StripeCustomerID
	} else {
		// Create a new customer
		params := &stripe.CustomerParams{
			Email: stripe.String(account.Email),
			Metadata: map[string]string{
				"account_id": account.ID,
			},
		}
		cust, err := customer.New(params)
		if err != nil {
			return "", err
		}
		if err := s.repo.UpdateStripeCustomerID(ctx, account.ID, cust.ID); err != nil {
			return "", err
		}
		customerID = cust.ID
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}
	sess, err := session.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// CreateCheckoutSession creates a Stripe Checkout session for a new subscription.
func (s *Service) CreateCheckoutSession(ctx context.Context, account *models.Account, priceID, returnURL string) (string, error) {
	var customerID string
	if account.StripeCustomerID != nil {
		customerID = *account.StripeCustomerID
	} else {
		// Create a new customer
		params := &stripe.CustomerParams{
			Email: stripe.String(account.Email),
			Metadata: map[string]string{
				"account_id": account.ID,
			},
		}
		cust, err := customer.New(params)
		if err != nil {
			return "", err
		}
		if err := s.repo.UpdateStripeCustomerID(ctx, account.ID, cust.ID); err != nil {
			return "", err
		}
		customerID = cust.ID
	}

	params := &stripe.CheckoutSessionParams{
		Customer:   stripe.String(customerID),
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(returnURL + "?success=true"),
		CancelURL:  stripe.String(returnURL + "?canceled=true"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
	}
	sess, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

type usageEvent struct {
	AccountID string  `json:"account_id"`
	VideoID   string  `json:"video_id"`
	Duration  float64 `json:"duration"`
}

// ConsumeUsageEvents listens to NATS and reports usage to Stripe and DB.
func (s *Service) ConsumeUsageEvents(ctx context.Context, nc *nats.Conn, log *logger.Logger) error {
	sub, err := nc.SubscribeSync("motionmesh.billing.usage")
	if err != nil {
		return fmt.Errorf("subscribe to usage events: %w", err)
	}
	defer sub.Unsubscribe()

	log.Info("Started consuming usage events on motionmesh.billing.usage")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := sub.NextMsg(5 * time.Second)
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				log.Error("nats next msg: %v", err)
				continue
			}

			var ev usageEvent
			if err := json.Unmarshal(msg.Data, &ev); err != nil {
				log.Error("unmarshal usage event: %v", err)
				continue
			}

			acc, err := s.repo.GetAccountByID(ctx, ev.AccountID)
			if err != nil || acc == nil {
				log.Error("failed to get account %s for usage reporting: %v", ev.AccountID, err)
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

			err = s.ReportUsage(ctx, ev.AccountID, "video_transcode_seconds", qty, stripeID)
			if err != nil {
				log.Error("failed to report usage for account %s: %v", ev.AccountID, err)
			} else {
				log.Info("Reported usage for account %s: %d seconds", ev.AccountID, qty)
			}
		}
	}
}
