package config

import (
	"os"
	"strconv"
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

	// Scalability & Performance
	LoadTestMode     bool
	WorkerConcurrency int
	AllowedOrigins       string
	CloudFrontDomain     string
	CloudFrontKeyID      string
	CloudFrontPrivateKey string
	DBMaxConns           int
	DBMinConns           int
	RateLimitEnabled     bool
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

		LoadTestMode:      getEnvBool("LOAD_TEST_MODE", false),
		WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 4),
		AllowedOrigins:       getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001"),
		CloudFrontDomain:     getEnv("CLOUDFRONT_DOMAIN", ""),
		CloudFrontKeyID:      getEnv("CLOUDFRONT_KEY_ID", ""),
		CloudFrontPrivateKey: getEnv("CLOUDFRONT_PRIVATE_KEY", ""),
		DBMaxConns:           getEnvInt("DB_MAX_CONNS", 25),
		DBMinConns:           getEnvInt("DB_MIN_CONNS", 5),
		RateLimitEnabled:     getEnvBool("RATE_LIMIT_ENABLED", true),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		return value == "true" || value == "1"
	}
	return fallback
}
