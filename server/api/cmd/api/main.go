package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	"github.com/motionmesh/server/api/internal/auth"
	authpostgres "github.com/motionmesh/server/api/internal/auth/postgres"
	"github.com/motionmesh/server/api/internal/billing"
	billingpostgres "github.com/motionmesh/server/api/internal/billing/postgres"
	"github.com/motionmesh/server/api/internal/branding"
	brandingpostgres "github.com/motionmesh/server/api/internal/branding/postgres"
	"github.com/motionmesh/server/api/internal/buckets"
	bucketspostgres "github.com/motionmesh/server/api/internal/buckets/postgres"
	apimiddleware "github.com/motionmesh/server/api/internal/middleware"

	"github.com/motionmesh/server/shared/storage"
	"github.com/motionmesh/server/api/internal/transcode"
	"github.com/motionmesh/server/api/internal/videos"
	videospostgres "github.com/motionmesh/server/api/internal/videos/postgres"
	sharedbranding "github.com/motionmesh/server/shared/branding"
	"github.com/motionmesh/server/shared/config"
	"github.com/motionmesh/server/shared/logger"
	"github.com/motionmesh/server/shared/models"
)

func main() {
	cfg := config.Load()
	log := logger.New()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Database ─────────────────────────────────────────────────────────────
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	// ── NATS ─────────────────────────────────────────────────────────────────
	nc, err := nats.Connect(cfg.QueueURL)
	if err != nil {
		log.Error("nats connect: %v", err)
		os.Exit(1)
	}
	defer nc.Drain()

	// ── Object Storage (Backblaze B2 via generic S3 API) ─────────────────────
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.StorageAccessKey,
			cfg.StorageSecretKey,
			"",
		)),
		awsconfig.WithRegion(cfg.StorageRegion),
	)
	if err != nil {
		log.Error("aws config: %v", err)
		os.Exit(1)
	}
	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.StorageEndpoint)
		o.UsePathStyle = true // required for B2 and most S3-compatible services
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	storageAdapter := storage.NewS3Adapter(s3Client, cfg.StorageBucket)
	if err := storageAdapter.CheckACL(ctx); err != nil {
		log.Error("storage bucket unreachable (continuing anyway): %v", err)
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdbOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Error("redis config: %v", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(rdbOpts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Error("redis connect: %v", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// ── Auth ──────────────────────────────────────────────────────────────────
	authRepo := authpostgres.NewRepository(db)
	authSvc := auth.NewService(authRepo, rdb, cfg.ClerkSecretKey, cfg.ClerkJWKSURL)

	// ── Billing ───────────────────────────────────────────────────────────────
	var billingRepo billing.BillingRepository = billingpostgres.NewRepository(db)
	billingSvc := billing.NewService(billingRepo, rdb, cfg.StripeSecretKey, cfg.StripeWebhookSecret)
	go func() { if err := billingSvc.ConsumeUsageEvents(ctx, nc, log); err != nil { log.Error("ConsumeUsageEvents failed: %v", err) } }()

	// ── Auth last-used flush ───────────────────────────────────────────────────
	// Drains the Redis buffer of api_key last-used timestamps to Postgres every
	// 5 minutes. Runs as a background goroutine; failed flushes are retried on
	// the next tick so no data is silently lost.
	go auth.FlushLastUsedLoop(ctx, rdb, authRepo, 5*time.Minute)

	// ── Buckets ───────────────────────────────────────────────────────────────
	var bucketRepo buckets.BucketRepository = bucketspostgres.NewRepository(db)
	bucketSvc := buckets.NewService(bucketRepo, storageAdapter)

	// ── Branding ──────────────────────────────────────────────────────────────
	var brandingRepo sharedbranding.BrandingRepository = brandingpostgres.NewRepository(db)
	brandingSvc := branding.NewService(brandingRepo, storageAdapter)

	// ── Videos ────────────────────────────────────────────────────────────────
	videosRepo := videospostgres.NewRepository(db)
	videosSvc := videos.NewService(videosRepo)
	transcodeSvc := transcode.NewService(db, nc)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	
	// Basic CORS setup
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.StripSlashes)

	// Public endpoints — no auth
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})



	// Protected routes — both Clerk JWT and mot_* API key are accepted
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authSvc,
			"/health",
			"/v1/billing/webhook",
			"/v1/videos/*/hls/*",
			"/v1/videos/*/hls",
		))
		// Auth / API keys
		r.Route("/v1/api-keys", func(r chi.Router) {
			authHandler := auth.NewHandler(authSvc)
			authHandler.RegisterRoutes(r)
		})

		r.Route("/v1/videos", func(r chi.Router) {
			videosHandler := videos.NewHandler(videosSvc, storageAdapter, transcodeSvc, bucketSvc, cfg.StorageBucket)
			videosHandler.RegisterRoutes(r)
		})

		// Jobs (transcode job status for the Media Converter dashboard)
		r.Get("/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
			acc, ok := r.Context().Value(auth.AccountContextKey).(*models.Account)
			if !ok || acc == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			limitStr := r.URL.Query().Get("limit")
			limit, _ := strconv.Atoi(limitStr)
			jobs, err := transcodeSvc.ListJobs(r.Context(), acc.ID, limit)
			if err != nil {
				log.Error("list jobs: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if jobs == nil {
				jobs = []*models.Job{}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jobs)
		})

		// Buckets
		r.Route("/v1/buckets", func(r chi.Router) {
			bucketsHandler := buckets.NewHandler(bucketSvc)
			bucketsHandler.RegisterRoutes(r)
		})

		// Branding (pro tier only)
		r.Route("/v1/branding", func(r chi.Router) {
			r.Use(apimiddleware.RequirePlan("pro", billingSvc))
			brandingHandler := branding.NewHandler(brandingSvc)
			brandingHandler.RegisterRoutes(r)
		})

		// Billing
		r.Route("/v1/billing", func(r chi.Router) {
			billingHandler := billing.NewHandler(billingSvc)
			billingHandler.RegisterRoutes(r)
		})
	})

	// ── Server ────────────────────────────────────────────────────────────────
	// ReadTimeout and WriteTimeout are intentionally unset (0 = no timeout) so that
	// large video proxy uploads are not killed mid-stream. Route-level timeouts can
	// be added via http.TimeoutHandler where needed.
	srv := &http.Server{
		Addr:        ":8080",
		Handler:     r,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Info("API server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
