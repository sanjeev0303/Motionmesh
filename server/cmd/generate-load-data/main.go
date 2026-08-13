package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Account struct {
	ID               string
	Email            string
	ClerkUserID      string
	ClerkOrgID       string
	StripeCustomerID string
	Plan             string
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Bucket struct {
	ID        string
	AccountID string
	Name      string
	CreatedAt time.Time
}

type Video struct {
	ID        string
	AccountID string
	BucketID  string
	ObjectKey string
	Title     string
	Status    string
	Duration  float32
	SizeBytes int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func main() {
	var (
		numAccounts int
		numVideos   int
	)

	flag.IntVar(&numAccounts, "accounts", 100, "Number of test accounts to generate")
	flag.IntVar(&numVideos, "videos", 1000, "Total number of test videos to generate and distribute among accounts")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/motionmesh?sslmode=disable" // fallback for local dev
	}

	ctx := context.Background()

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("failed to parse db config: %v", err)
	}

	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("database connect: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("database ping: %v", err)
	}

	log.Printf("Connected to database. Generating %d accounts and %d total videos...", numAccounts, numVideos)

	startTime := time.Now()

	// 1. Generate Accounts
	accounts := make([]Account, numAccounts)
	buckets := make([]Bucket, numAccounts)
	now := time.Now()

	accountRows := make([][]any, numAccounts)
	bucketRows := make([][]any, numAccounts)

	for i := 0; i < numAccounts; i++ {
		accID := uuid.New().String()
		accounts[i] = Account{
			ID:               accID,
			Email:            fmt.Sprintf("loadtest-user-%d-%s@example.com", i, accID[:8]),
			ClerkUserID:      "user_" + accID,
			ClerkOrgID:       "org_" + accID,
			StripeCustomerID: "cus_" + accID[:14],
			Plan:             "free",
			Status:           "active",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		accountRows[i] = []any{
			accounts[i].ID,
			accounts[i].Email,
			accounts[i].ClerkUserID,
			accounts[i].ClerkOrgID,
			accounts[i].StripeCustomerID,
			accounts[i].Plan,
			accounts[i].Status,
			accounts[i].CreatedAt,
			accounts[i].UpdatedAt,
		}

		bucketID := uuid.New().String()
		buckets[i] = Bucket{
			ID:        bucketID,
			AccountID: accID,
			Name:      fmt.Sprintf("loadtest-bucket-%s", accID[:8]),
			CreatedAt: now,
		}

		bucketRows[i] = []any{
			buckets[i].ID,
			buckets[i].AccountID,
			buckets[i].Name,
			buckets[i].CreatedAt,
		}
	}

	// Insert accounts
	_, err = db.CopyFrom(
		ctx,
		pgx.Identifier{"accounts"},
		[]string{"id", "email", "clerk_user_id", "clerk_org_id", "stripe_customer_id", "plan", "status", "created_at", "updated_at"},
		pgx.CopyFromRows(accountRows),
	)
	if err != nil {
		log.Fatalf("failed to insert accounts: %v", err)
	}
	log.Printf("Successfully inserted %d accounts.", numAccounts)

	// Insert buckets
	_, err = db.CopyFrom(
		ctx,
		pgx.Identifier{"buckets"},
		[]string{"id", "account_id", "name", "created_at"},
		pgx.CopyFromRows(bucketRows),
	)
	if err != nil {
		log.Fatalf("failed to insert buckets: %v", err)
	}
	log.Printf("Successfully inserted %d buckets.", numAccounts)

	// 2. Generate Videos
	if numVideos > 0 && numAccounts > 0 {
		videoRows := make([][]any, numVideos)
		for i := 0; i < numVideos; i++ {
			// Randomly assign to an account
			idx := rand.Intn(numAccounts)
			acc := accounts[idx]
			bkt := buckets[idx]
			vidID := uuid.New().String()

			videoRows[i] = []any{
				vidID,
				acc.ID,
				bkt.ID,
				fmt.Sprintf("raw/%s.mp4", vidID),
				fmt.Sprintf("Load Test Video %d", i),
				"queued",
				float32(rand.Intn(3600)),       // up to 1 hour
				int64(rand.Intn(1024*1024*500)), // up to 500MB
				now,
				now,
			}
		}

		// Because pgx.CopyFrom handles thousands of rows easily, we can insert all videos at once if it's not too huge (e.g. 1M rows).
		// For very large numbers, we should chunk it. Let's chunk every 10,000 rows.
		chunkSize := 10000
		for i := 0; i < len(videoRows); i += chunkSize {
			end := i + chunkSize
			if end > len(videoRows) {
				end = len(videoRows)
			}

			_, err = db.CopyFrom(
				ctx,
				pgx.Identifier{"videos"},
				[]string{"id", "account_id", "bucket_id", "object_key", "title", "status", "duration", "size_bytes", "created_at", "updated_at"},
				pgx.CopyFromRows(videoRows[i:end]),
			)
			if err != nil {
				log.Fatalf("failed to insert videos chunk %d-%d: %v", i, end, err)
			}
		}
		log.Printf("Successfully inserted %d videos.", numVideos)
	}

	duration := time.Since(startTime)
	log.Printf("Load data generation complete in %s.", duration)
}
