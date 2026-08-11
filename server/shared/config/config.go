package config

import (
	"os"
)

type Config struct {
	// Core infrastructure
	DatabaseURL string
	RedisURL    string
	QueueURL    string

	// Object storage — one code path, different values per environment
	StorageEndpoint  string
	StorageAccessKey string
	StorageSecretKey string
	StorageBucket    string
	StorageRegion    string
	StorageUseSSL    bool

	// Auth
	ClerkSecretKey string
	ClerkJWKSURL   string
	JWTSecret      string

	// Billing
	StripeSecretKey      string
	StripeWebhookSecret  string

	// Worker / Sidecar
	CaptionsSidecarURL string
	AnthropicAPIKey    string
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/motionmesh?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),
		QueueURL:    getEnv("QUEUE_URL", "nats://localhost:4222"),

		StorageEndpoint:  getEnv("STORAGE_ENDPOINT", ""),
		StorageAccessKey: getEnv("STORAGE_ACCESS_KEY", ""),
		StorageSecretKey: getEnv("STORAGE_SECRET_KEY", ""),
		StorageBucket:    getEnv("STORAGE_BUCKET", "motionmesh-dev"),
		StorageRegion:    getEnv("STORAGE_REGION", "us-east-005"),
		StorageUseSSL:    getEnv("STORAGE_USE_SSL", "true") == "true",

		ClerkSecretKey: getEnv("CLERK_SECRET_KEY", ""),
		ClerkJWKSURL:   getEnv("CLERK_JWKS_URL", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),

		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),

		CaptionsSidecarURL: getEnv("CAPTIONS_SIDECAR_URL", "http://localhost:8000"),
		AnthropicAPIKey:    getEnv("ANTHROPIC_API_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
