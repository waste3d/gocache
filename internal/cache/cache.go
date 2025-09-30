package cache

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("key not found")

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}) error
	Delete(key string) error
}

type item struct {
	Value interface{}
}

type memoryCache struct {
	mu    sync.RWMutex
	items map[string]*item
}

func New() Cache {
	return &memoryCache{items: make(map[string]*item)}
}

func (m *memoryCache) Get(key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, found := m.items[key]
	if !found {
		return nil, ErrNotFound
	}

	return item.Value, nil
}

func (m *memoryCache) Set(key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = &item{Value: value}
	return nil
}

func (m *memoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
	return nil
}
