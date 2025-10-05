package main

import (
	"errors"
	"flag"
	"gocache/cmd/server"
	"gocache/internal/cache"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const defaultShardCount = 32

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Starting GoCache server...")

	port := flag.String("port", "6379", "Port to listen on")
	shards := flag.Int("shards", defaultShardCount, "Number of shards to use")
	maxSize := flag.Int("max-size", 10000, "Max number of items in cache (total)")
	cleanupInterval := flag.Duration("cleanup-interval", 10*time.Second, "Interval for cleaning up expired keys")

	flag.Parse()

	c := cache.NewShardedCache(uint32(*shards), *maxSize, *cleanupInterval)

	if err := c.LoadFromFile("dump.goc"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// Если это любая другая ошибка - логируем и вызываем os.Exit(1)
			log.Fatalf("Failed to load data from file: %v", err)
		}

		// Если это os.ErrNotExist, мы просто продолжаем с пустым кэшем
		log.Println("No dump file found, starting with an empty cache.")
	} else {
		log.Println("Cache data loaded from dump.gob.")
	}

	srv := server.New(c)
	if err := srv.Listen(":" + *port); err != nil {
		log.Fatalf("Failed to listen on port %v: %v", port, err)
	}

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server runtime error: %v", err)
		}
	}()

	log.Printf("Server is ready to accept connections at %v", *port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Shutting down server...")

	log.Println("Saving cache data to dump.goc...")
	if err := c.SaveToFile("dump.goc"); err != nil {
		log.Printf("ERROR: Failed to save cache data: %v", err)
	} else {
		log.Println("Cache data saved successfully.")
	}

	srv.Stop()
	c.Stop()
	log.Println("Server gracefully stopped.")
}
