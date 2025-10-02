package cache

import (
	"container/list"
	"encoding/gob"
	"errors"
	"os"
	"sync"
	"time"
)

var ErrNotFound = errors.New("key not found")

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	SaveToFile(path string) error
	LoadFromFile(path string) error
	Stop()
}

type item struct {
	Key        string
	Value      interface{}
	Expiration int64
}

type inMemoryCache struct {
	mu      sync.RWMutex
	items   map[string]*list.Element
	ll      *list.List
	maxSize int
	stopCh  chan struct{}
}

func New(cleanupInterval time.Duration, maxSize int) Cache {
	c := &inMemoryCache{
		items:   make(map[string]*list.Element),
		stopCh:  make(chan struct{}),
		maxSize: maxSize,
		ll:      list.New(),
	}

	if cleanupInterval > 0 {
		go c.cleanupLoop(cleanupInterval)
	}

	return c
}

func (m *inMemoryCache) deleteExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UnixNano()
	var keysToDelete []string

	for key, elem := range m.items {
		it := elem.Value.(*item)

		if it.Expiration > 0 && it.Expiration < now {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		if elem, ok := m.items[key]; ok {
			m.ll.Remove(elem)
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

func (m *inMemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiration int64
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	if elem, ok := m.items[key]; ok {
		m.ll.MoveToFront(elem)

		it := elem.Value.(*item)
		it.Expiration = expiration
		it.Value = value
	} else {
		it := &item{Key: key, Value: value, Expiration: expiration}

		elem := m.ll.PushFront(it)

		m.items[key] = elem
	}

	if m.maxSize > 0 && m.ll.Len() > m.maxSize {
		lruElement := m.ll.Back()
		if lruElement != nil {
			m.ll.Remove(lruElement)

			lruItem := lruElement.Value.(*item)

			delete(m.items, lruItem.Key)
		}
	}

	return nil
}

func (m *inMemoryCache) Get(key string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if elem, ok := m.items[key]; ok {
		it := elem.Value.(*item)

		if it.Expiration > 0 && it.Expiration < time.Now().UnixNano() {
			m.ll.Remove(elem)
			delete(m.items, key)
			return nil, ErrNotFound
		}

		m.ll.MoveToFront(elem)
		return it.Value, nil
	}

	return nil, ErrNotFound
}

func (m *inMemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if elem, ok := m.items[key]; ok {
		m.ll.Remove(elem)

		delete(m.items, key)
	}

	return nil
}

func (m *inMemoryCache) SaveToFile(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	itemsToSave := make(map[string]*item)

	for e := m.ll.Front(); e != nil; e = e.Next() {
		it := e.Value.(*item)
		itemsToSave[it.Key] = it
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(itemsToSave); err != nil {
		return err
	}

	return nil
}

func (m *inMemoryCache) LoadFromFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var itemsToLoad map[string]*item
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&itemsToLoad); err != nil {
		return err
	}

	m.items = make(map[string]*list.Element)
	m.ll = list.New()

	for key, it := range itemsToLoad {
		elem := m.ll.PushFront(it)
		m.items[key] = elem
	}
	return nil
}
