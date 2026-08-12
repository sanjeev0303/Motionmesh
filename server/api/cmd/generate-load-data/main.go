package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motionmesh/server/api/internal/auth"
	authpostgres "github.com/motionmesh/server/api/internal/auth/postgres"
)

type OutputData struct {
	APIKeys []string `json:"api_keys"`
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = crypto_rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	log.Println("Connecting...")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}
	defer pool.Close()
	log.Println("Connected!")

	authRepo := authpostgres.NewRepository(pool)
	authSvc := auth.NewService(authRepo, nil, "dummy_secret", "")

	var keys []string

	for i := 0; i < 10; i++ {
		log.Printf("Iteration %d...", i)
		accountID := newUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO accounts (id, email, plan, status) 
			VALUES ($1, $2, 'enterprise', 'active') 
			ON CONFLICT (id) DO NOTHING`,
			accountID, fmt.Sprintf("test_%s@example.com", accountID[:8]))
		if err != nil {
			log.Fatalf("Failed to insert account: %v", err)
		}
		log.Println("Inserted account")

		rawKey, _, err := authSvc.GenerateAPIKey(ctx, accountID, fmt.Sprintf("Key %d", i))
		if err != nil {
			log.Fatalf("Failed to generate API key: %v", err)
		}
		keys = append(keys, rawKey)
		log.Println("Generated API key")

		bucketID := newUUID()
		_, err = pool.Exec(ctx, `
			INSERT INTO buckets (id, account_id, name) 
			VALUES ($1, $2, $3) 
			ON CONFLICT (id) DO NOTHING`,
			bucketID, accountID, fmt.Sprintf("bucket-%s", bucketID[:8]))
		if err != nil {
			log.Fatalf("Failed to insert bucket: %v", err)
		}
		log.Println("Inserted bucket")

		for v := 0; v < 100; v++ {
			videoID := newUUID()
			_, err = pool.Exec(ctx, `
				INSERT INTO videos (id, account_id, bucket_id, object_key, title, status, duration, size_bytes) 
				VALUES ($1, $2, $3, $4, $5, 'ready', 120, 1024000) 
				ON CONFLICT (id) DO NOTHING`,
				videoID, accountID, bucketID, fmt.Sprintf("videos/%s.mp4", videoID), fmt.Sprintf("Video %d", v))
			if err != nil {
				log.Fatalf("Failed to insert video: %v", err)
			}
		}
		log.Println("Inserted videos")
	}

	out := OutputData{APIKeys: keys}
	data, _ := json.MarshalIndent(out, "", "  ")
	err = os.WriteFile("../../tests/load/k6/data.json", data, 0644)
	if err != nil {
		log.Fatalf("Failed to write data.json: %v", err)
	}

	log.Println("Successfully generated load test data to tests/load/k6/data.json")
}
