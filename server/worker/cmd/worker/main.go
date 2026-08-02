package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"database/sql"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"

	brandingpostgres "github.com/motionmesh/server/shared/branding/postgres"
	"github.com/motionmesh/server/shared/config"
	"github.com/motionmesh/server/shared/logger"
	"github.com/motionmesh/server/shared/storage"
	"github.com/motionmesh/server/worker/internal/captions"
	"github.com/motionmesh/server/worker/internal/job"
	"github.com/motionmesh/server/worker/internal/uploader"
)

func main() {
	cfg := config.Load()
	log := logger.New()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("Starting Worker on Queue: %s", cfg.QueueURL)

	// ── Database ─────────────────────────────────────────────────────────────
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect: %v", err)
		os.Exit(1)
	}
	defer db.Close()
	
	if err := db.PingContext(ctx); err != nil {
		log.Error("database ping: %v", err)
		os.Exit(1)
	}

	// ── NATS ─────────────────────────────────────────────────────────────────
	nc, err := nats.Connect(cfg.QueueURL)
	if err != nil {
		log.Error("nats connect: %v", err)
		os.Exit(1)
	}
	defer nc.Drain()

	// ── Object Storage ───────────────────────────────────────────────────────
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
		o.UsePathStyle = true
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	storageAdapter := storage.NewS3Adapter(s3Client, cfg.StorageBucket)
	if err := storageAdapter.CheckACL(ctx); err != nil {
		log.Error("storage bucket unreachable: %v", err)
		os.Exit(1)
	}

	// ── Components ───────────────────────────────────────────────────────────
	brandingRepo := brandingpostgres.NewRepository(db)
	up := uploader.NewUploader(storageAdapter)
	capClient := captions.NewClient(cfg.CaptionsSidecarURL, &http.Client{Timeout: 30 * time.Minute})
	
	jobHandler := job.NewHandler(db, storageAdapter, up, capClient, brandingRepo, log, nc)
	consumer := job.NewConsumer(nc, jobHandler, log)

	// ── Start Consumer ───────────────────────────────────────────────────────
	if err := consumer.Start(ctx); err != nil {
		log.Error("consumer failed: %v", err)
	}

	log.Info("shutting down...")
}
