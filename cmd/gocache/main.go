package main

import (
	"gocache/cmd/server"
	"gocache/internal/cache"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Starting GoCache server...")

	c := cache.New(10 * time.Second)

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

	srv.Stop()
	c.Stop()
	log.Println("Server gracefully stopped.")
}
