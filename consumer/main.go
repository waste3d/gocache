package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	kafkaBroker     = "localhost:9092"
	kafkaTopic      = "logs"
	consumerGroupID = "log-processors"
	numWorkers      = 3
)

func worker(id int, wg *sync.WaitGroup, reader *kafka.Reader) {
	defer wg.Done()
	log.Printf("Worker %d started", id)

	for {
		msg, err := reader.FetchMessage(context.Background())
		if err != nil {
			log.Printf("Worker %d failed to fetch message: %s", id, err)
			break
		}
		log.Printf("Worker %d: received message on partition %d, offset %d: %s", id, msg.Partition, msg.Offset, string(msg.Value))
		time.Sleep(1 * time.Second)

		if err := reader.CommitMessages(context.Background(), msg); err != nil {
			log.Printf("Worker %d: failed to commit message: %v", id, err)
		}
	}
	log.Printf("Worker %d finished", id)
}
