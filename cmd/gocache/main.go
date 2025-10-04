package main

import (
	"errors"
	"gocache/cmd/server"
	"gocache/internal/cache"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const defaultShardCount = 32

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Starting GoCache server...")

	c := cache.NewShardedCache(defaultShardCount)

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
	if err := srv.Listen(":6379"); err != nil {
		log.Fatalf("Failed to listen on port 6379: %v", err)
	}

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server runtime error: %v", err)
		}
	}()

	log.Println("Server is ready to accept connections at :6379")

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
