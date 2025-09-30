package cache

import (
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("key not found")

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	Stop()
}

type item struct {
	Value      interface{}
	Expiration int64
}

type inMemoryCache struct {
	mu     sync.RWMutex
	items  map[string]*item
	stopCh chan struct{}
}

func New(cleanupInterval time.Duration) Cache {
	c := &inMemoryCache{
		items:  make(map[string]*item),
		stopCh: make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go c.cleanupLoop(cleanupInterval)
	}

	return c
}

func (m *inMemoryCache) deleteExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, item := range m.items {
		if item.Expiration < time.Now().UnixNano() {
			delete(m.items, key)
		}
	}
}

func (m *inMemoryCache) Stop() {
	close(m.stopCh)
}

func (m *inMemoryCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.deleteExpired()
		case <-m.stopCh:
			ticker.Stop()
			return
		}
	}
}

func (m *inMemoryCache) Get(key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, found := m.items[key]
	if !found {
		return nil, ErrNotFound
	}

	// Реализуем пассивное вытеснение
	if item.Expiration < time.Now().UnixNano() && item.Expiration > 0 {
		return nil, ErrNotFound
	}

	return item.Value, nil
}

func (m *inMemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	var expiration int64

	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = &item{
		Value:      value,
		Expiration: expiration,
	}
	return nil
}

func (m *inMemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
	return nil
}
