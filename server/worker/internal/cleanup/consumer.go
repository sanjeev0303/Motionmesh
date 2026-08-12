package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/motionmesh/server/shared/logger"
	"github.com/motionmesh/server/shared/storage"
	"github.com/nats-io/nats.go"
)

type Consumer struct {
	nc      *nats.Conn
	storage storage.ObjectStorage
	log     *logger.Logger
}

func NewConsumer(nc *nats.Conn, storage storage.ObjectStorage, log *logger.Logger) *Consumer {
	return &Consumer{
		nc:      nc,
		storage: storage,
		log:     log,
	}
}

type VideoCleanupMessage struct {
	VideoID      string  `json:"video_id"`
	ObjectKey    string  `json:"object_key"`
	ThumbnailKey *string `json:"thumbnail_key"`
	SpriteKey    *string `json:"sprite_key"`
	PreviewKey   *string `json:"preview_key"`
}

func (c *Consumer) Start(ctx context.Context) error {
	js, err := c.nc.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get jetstream context: %w", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "CLEANUP",
		Subjects: []string{"video.cleanup"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		c.log.Error("failed to add cleanup stream (might already exist): %v", err)
	}

	_, err = js.AddConsumer("CLEANUP", &nats.ConsumerConfig{
		Durable:       "cleanup_worker",
		AckPolicy:     nats.AckExplicitPolicy,
		MaxDeliver:    5,
		AckWait:       5 * time.Minute,
		FilterSubject: "video.cleanup",
	})
	if err != nil {
		c.log.Error("failed to add cleanup consumer (might already exist): %v", err)
	}

	sub, err := js.PullSubscribe("video.cleanup", "cleanup_worker")
	if err != nil {
		return fmt.Errorf("failed to pull subscribe cleanup: %w", err)
	}

	c.log.Info("Started NATS consumer for video.cleanup")

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Cleanup consumer shutting down")
			return nil
		default:
			msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil {
				if err != nats.ErrTimeout {
					c.log.Error("cleanup fetch error: %v", err)
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
	var payload VideoCleanupMessage
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		c.log.Error("failed to unmarshal cleanup message: %v", err)
		msg.Term()
		return
	}

	c.log.Info("Processing cleanup for video %s", payload.VideoID)

	keysToDelete := []string{payload.ObjectKey}
	if payload.ThumbnailKey != nil && *payload.ThumbnailKey != "" {
		keysToDelete = append(keysToDelete, *payload.ThumbnailKey)
	}
	if payload.SpriteKey != nil && *payload.SpriteKey != "" {
		keysToDelete = append(keysToDelete, *payload.SpriteKey)
	}
	if payload.PreviewKey != nil && *payload.PreviewKey != "" {
		keysToDelete = append(keysToDelete, *payload.PreviewKey)
	}

	for _, key := range keysToDelete {
		if key != "" {
			if err := c.storage.DeleteObject(ctx, key); err != nil {
				c.log.Error("failed to delete storage key %s for video %s: %v", key, payload.VideoID, err)
			}
		}
	}

	c.log.Info("Cleanup completed successfully for video %s", payload.VideoID)
	msg.Ack()
}
