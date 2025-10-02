package server

import (
	"bufio"
	"errors"
	"fmt"
	"gocache/internal/cache"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	cache    cache.Cache
	listener net.Listener
	wg       sync.WaitGroup
}

func New(c cache.Cache) *Server {
	return &Server{
		cache: c,
	}
}

func (s *Server) Listen(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.listener = listener

	return nil

}

func (s *Server) Start() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("Accept loop gracefully stopped.")
				return nil
			}
			return err
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Printf("Error closing connection: %v", err)
		}
	}()

	scanner := bufio.NewScanner(conn)

ConnectionLoop:
	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.Fields(line)
		if len(parts) == 0 {
			fmt.Println("Empty line")
			continue
		}

		command := strings.ToUpper(parts[0])

		if len(parts) < 1 {
			fmt.Println("Not enough parts (min 1)")
		}

		switch command {
		case "GET":
			value, err := s.cache.Get(parts[1])
			if err != nil {
				io.WriteString(conn, "(nil)\n")
			} else {
				fmt.Fprintf(conn, "%v\n", value)
			}
		case "SET":
			if len(parts) < 3 {
				fmt.Println("Not enough parts (min 3)")
				continue
			}

			key := parts[1]
			value := parts[2]
			var ttl time.Duration = 0

			if len(parts) == 4 {
				ttlInSeconds, err := strconv.ParseInt(parts[3], 10, 64)
				if err != nil {
					log.Printf("Error parsing TTL: %v", err)
				}
				ttl = time.Duration(ttlInSeconds) * time.Second
			}
			if err := s.cache.Set(key, value, ttl); err != nil {
				log.Printf("Error setting value: %v", err)
			} else {
				fmt.Println("Set OK")
			}
		case "DEL":
			if len(parts) != 2 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for '%s' command\n", parts[0])
				continue
			}
			key := parts[1]
			_, err := s.cache.Get(key)
			if err != nil {
				io.WriteString(conn, "Key not found\n") // Ключа не было
			} else {
				s.cache.Delete(key)
				io.WriteString(conn, "Deleted successfully\n") // Ключ был и удален
			}
		case "CLEAR":
			if len(parts) != 1 {
				fmt.Fprintf(conn, "ERROR: 'CLEAR' command takes no arguments\n")
				continue
			}
			io.WriteString(conn, "\x1b[2J\x1b[H\u001B[2J\u001B[H") // Очищаем строку
		case "EXIT":
			break ConnectionLoop // Выходим из цикла
		default:
			fmt.Fprintf(conn, "ERROR: unknown command '%s'\n", command)
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(conn, "ERROR: %v\n", err)
		}
	}

}

func (s *Server) Stop() {
	log.Println("Stopping server...")
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	log.Println("All connections closed.")
}
