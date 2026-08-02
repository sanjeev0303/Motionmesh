package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/motionmesh/server/shared/logger"
	"github.com/nats-io/nats.go"
)

type Consumer struct {
	nc      *nats.Conn
	handler *Handler
	log     *logger.Logger
}

func NewConsumer(nc *nats.Conn, handler *Handler, log *logger.Logger) *Consumer {
	return &Consumer{
		nc:      nc,
		handler: handler,
		log:     log,
	}
}

type TranscodeJobMessage struct {
	VideoID           string  `json:"video_id"`
	SourceObjectKey   string  `json:"source_object_key"`
	TranscodeBucketID *string `json:"transcode_bucket_id,omitempty"`
}

func (c *Consumer) Start(ctx context.Context) error {
	js, err := c.nc.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get jetstream context: %w", err)
	}

	// Ensure stream exists
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "TRANSCODE",
		Subjects: []string{"transcode.jobs"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		c.log.Error("failed to add stream (might already exist): %v", err)
	}

	// Ensure consumer exists
	// We use a pull consumer to control concurrency per worker
	_, err = js.AddConsumer("TRANSCODE", &nats.ConsumerConfig{
		Durable:       "transcode_worker",
		AckPolicy:     nats.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       30 * time.Minute, // transcode can take a long time
		FilterSubject: "transcode.jobs",
	})
	if err != nil {
		c.log.Error("failed to add consumer (might already exist): %v", err)
	}

	sub, err := js.PullSubscribe("transcode.jobs", "transcode_worker")
	if err != nil {
		return fmt.Errorf("failed to pull subscribe: %w", err)
	}

	c.log.Info("Started NATS consumer for transcode.jobs")

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Consumer shutting down")
			return nil
		default:
			msgs, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
			if err != nil {
				if err != nats.ErrTimeout {
					c.log.Error("fetch error: %v", err)
				}
				continue
			}

			for _, msg := range msgs {
				c.handleMessage(ctx, msg)
			}
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg *nats.Msg) {
	var payload TranscodeJobMessage
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		c.log.Error("failed to unmarshal message: %v", err)
		msg.Term() // Terminal error, don't retry
		return
	}

	c.log.Info("Processing job for video %s", payload.VideoID)

	jobCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()

	err := c.handler.Process(jobCtx, payload.VideoID, payload.SourceObjectKey, payload.TranscodeBucketID)
	if err != nil {
		c.log.Error("job failed for video %s: %v", payload.VideoID, err)
		msg.Term() // Terminal error, job is marked as failed in DB, don't retry automatically
		return
	}

	c.log.Info("Job completed successfully for video %s", payload.VideoID)
	msg.Ack()
}
