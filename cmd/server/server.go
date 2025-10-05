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
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

ConnectionLoop:
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 0 {
			continue
		}

		command := strings.ToUpper(parts[0])

		switch command {
		case "GET":
			if len(parts) != 2 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'GET'\n")
				continue
			}
			key := parts[1]
			value, err := s.cache.Get(key)
			if err != nil {
				io.WriteString(conn, "(nil)\n")
			} else {
				fmt.Fprintf(conn, "%v\n", value)
			}

		case "SET":
			if len(parts) < 3 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'SET'\n")
				continue
			}
			key := parts[1]
			valueStr := parts[2]
			var ttl time.Duration = 0

			var valueToStore interface{}
			intValue, err := strconv.ParseInt(valueStr, 10, 64)
			if err == nil {
				valueToStore = intValue
			} else {
				valueToStore = valueStr
			}

			if len(parts) == 4 {
				ttlInSeconds, err := strconv.ParseInt(parts[3], 10, 64)
				if err != nil {
					fmt.Fprintf(conn, "ERROR: TTL must be an integer\n")
					continue
				}
				ttl = time.Duration(ttlInSeconds) * time.Second
			}

			// Передаем в кэш значение правильного типа
			if err := s.cache.Set(key, valueToStore, ttl); err != nil {
				fmt.Fprintf(conn, "ERROR: %v\n", err)
			} else {
				io.WriteString(conn, "OK\n")
			}
		case "DELETE":
			if len(parts) != 2 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'DELETE'\n")
				continue
			}
			key := parts[1]
			// Метод Delete ничего не возвращает, поэтому сначала проверяем наличие
			_, err := s.cache.Get(key)
			s.cache.Delete(key)
			if err != nil {
				io.WriteString(conn, "0\n") // Не было ключа
			} else {
				io.WriteString(conn, "1\n") // Ключ был
			}

		case "INCR":
			if len(parts) != 2 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'INCR'\n")
				continue
			}
			key := parts[1]
			newValue, err := s.cache.Incr(key)
			if err != nil {
				fmt.Fprintf(conn, "ERROR: %v\n", err)
			} else {
				fmt.Fprintf(conn, "%d\n", newValue) // Используем Fprintf для добавления \n
			}

		case "DECR":
			if len(parts) != 2 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'DECR'\n")
				continue // <-- БЫЛА ОШИБКА
			}
			key := parts[1]
			newValue, err := s.cache.Decr(key)
			if err != nil {
				fmt.Fprintf(conn, "ERROR: %v\n", err)
			} else {
				fmt.Fprintf(conn, "%d\n", newValue)
			}

		case "CLEAR":
			if len(parts) != 1 {
				fmt.Fprintf(conn, "ERROR: 'CLEAR' command takes no arguments\n")
				continue
			}
			io.WriteString(conn, "\x1b[2J\x1b[H")

		case "EXIT", "QUIT":
			break ConnectionLoop

		case "PING":
			io.WriteString(conn, "PONG\n")
		case "INFO":
			if len(parts) != 1 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'INFO'\n")
			}
			stats := s.cache.Info()
			var response strings.Builder
			for k, v := range stats {
				response.WriteString(fmt.Sprintf("%s:%s\n", k, v))
			}
			io.WriteString(conn, response.String())

		case "CONFIG":
			if len(parts) < 3 {
				fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'CONFIG'\n")
				continue
			}
			subcommand := strings.ToUpper(parts[1])
			switch subcommand {
			case "GET":
				if len(parts) != 3 {
					fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'CONFIG GET'\n")
					continue
				}
				param := parts[2]
				config := s.cache.GetConfig()
				if value, ok := config[param]; ok {
					fmt.Fprintf(conn, "%s:%s\n", param, value)
				} else {
					fmt.Fprintf(conn, "ERROR: unknown config parameter '%s'\n", param)
				}
			case "SET":
				if len(parts) != 4 {
					fmt.Fprintf(conn, "ERROR: wrong number of arguments for 'CONFIG SET'\n")
					continue
				}
				param := parts[2]
				value := parts[3]
				if err := s.cache.SetConfig(param, value); err != nil {
					fmt.Fprintf(conn, "ERROR: %v\n", err)
				} else {
					io.WriteString(conn, "OK\n")
				}
			default:
				fmt.Fprintf(conn, "ERROR: unknown command '%s'\n", command)
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error reading from connection: %v", err)
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
