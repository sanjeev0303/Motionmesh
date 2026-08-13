package main

import (
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	log.Printf("Connecting to NATS at %s...", natsURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to get JetStream context: %v", err)
	}

	streams := []struct {
		name     string
		subjects []string
		storage  nats.StorageType
	}{
		{"CLEANUP", []string{"video.cleanup"}, nats.FileStorage},
		{"TRANSCODE", []string{"transcode.jobs"}, nats.FileStorage},
		{"BILLING", []string{"motionmesh.billing.usage"}, nats.FileStorage},
	}

	for _, s := range streams {
		log.Printf("Provisioning stream: %s", s.name)
		_, err := js.AddStream(&nats.StreamConfig{
			Name:     s.name,
			Subjects: s.subjects,
			Storage:  s.storage,
		})
		if err != nil {
			log.Printf("Failed to add stream %s (might already exist): %v", s.name, err)
		} else {
			log.Printf("Stream %s added successfully", s.name)
		}
	}

	consumers := []struct {
		stream    string
		durable   string
		subject   string
		ackWait   time.Duration
		maxDeliv  int
	}{
		{"CLEANUP", "cleanup_worker", "video.cleanup", 5 * time.Minute, 5},
		{"TRANSCODE", "transcode_worker", "transcode.jobs", 30 * time.Minute, 3},
		{"BILLING", "billing_usage_worker", "motionmesh.billing.usage", 30 * time.Second, 5},
	}

	for _, c := range consumers {
		log.Printf("Provisioning consumer: %s on stream %s", c.durable, c.stream)
		_, err := js.AddConsumer(c.stream, &nats.ConsumerConfig{
			Durable:       c.durable,
			AckPolicy:     nats.AckExplicitPolicy,
			MaxDeliver:    c.maxDeliv,
			AckWait:       c.ackWait,
			FilterSubject: c.subject,
		})
		if err != nil {
			log.Printf("Failed to add consumer %s (might already exist): %v", c.durable, err)
		} else {
			log.Printf("Consumer %s added successfully", c.durable)
		}
	}

	log.Println("NATS infrastructure provisioning complete.")
}
