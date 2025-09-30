package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

type LogEntry struct {
	Level     string    `json:"level"`
	UserID    string    `json:"user_id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	logLevels = []string{"INFO", "WARNING", "ERROR", "FATAL", "DEBUG"}
	logEvents = []string{"login", "logout", "payment_success", "payment_failed", "item_added_to_cart"}
)

func main() {
	writer := kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "logs",
		Balancer: &kafka.LeastBytes{},
	}
	defer func() {
		err := writer.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	log.Println("Starting producer...")
	ctx := context.Background()

	for {
		entry := LogEntry{
			Level:     logLevels[rand.Intn(len(logLevels))],
			UserID:    "user-" + strconv.Itoa(rand.Intn(1000)),
			Event:     logEvents[rand.Intn(len(logEvents))],
			Timestamp: time.Now(),
		}

		msgBytes, err := json.Marshal(entry)
		if err != nil {
			log.Printf("could not marshal json: %v", err)
			continue
		}

		err = writer.WriteMessages(ctx, kafka.Message{
			Value: msgBytes,
		})
		if err != nil {
			log.Printf("could not write messages: %v", err)
		} else {
			log.Printf("Sent message: %s", string(msgBytes))
		}

		time.Sleep(1 * time.Second)
	}
}
