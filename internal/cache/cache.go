package cache

import (
	"container/list"
	"errors"
	"hash/fnv"
	_ "hash/fnv"
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

type cacheShard struct {
	mu      *sync.RWMutex
	items   map[string]*list.Element
	ll      *list.List
	maxSize int
	stopCh  chan struct{}
}

type ShardedCache struct {
	shards     []*cacheShard
	shardCount uint32
}

type item struct {
	Key        string
	Value      interface{}
	Expiration int64
}

func NewShardedCache(shardCount uint32, totalMaxSize int, clenupInterval time.Duration) *ShardedCache {
	sc := &ShardedCache{
		shards:     make([]*cacheShard, shardCount),
		shardCount: shardCount,
	}

	shardMaxCount := 1
	if totalMaxSize > 0 {
		shardMaxCount = totalMaxSize / int(shardCount)
		if shardMaxCount < 1 {
			shardMaxCount = 1
		}
	}

	for i := 0; i < int(shardCount); i++ {
		sc.shards[i] = &cacheShard{
			items:   make(map[string]*list.Element),
			mu:      new(sync.RWMutex),
			ll:      list.New(),
			maxSize: shardMaxCount,
			stopCh:  make(chan struct{}),
		}

		go sc.shards[i].cleanupLoop(clenupInterval)
	}

	return sc
}

func (c *cacheShard) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-c.stopCh:
			defer ticker.Stop()
			return
		}
	}
}

func (c *cacheShard) deleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	var keysToDelete []string

	// Шаг 1: Сбор ключей
	for key, elem := range c.items {
		it := elem.Value.(*item)
		if it.Expiration > 0 && it.Expiration < now {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Шаг 2: Удаление
	for _, key := range keysToDelete {
		if elem, ok := c.items[key]; ok {
			c.ll.Remove(elem)
			delete(c.items, key)
		}
	}
}

func (c *cacheShard) set(key string, value interface{}, expiration int64) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	// Сценарий 1: Обновление
	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		it := elem.Value.(*item)
		it.Value = value
		it.Expiration = expiration
	} else {
		// Сценарий 2: Добавление
		it := &item{Key: key, Value: value, Expiration: expiration}
		elem := c.ll.PushFront(it)
		c.items[key] = elem
	}

	// Проверка и вытеснение (сразу после добавления)
	if c.maxSize > 0 && c.ll.Len() > c.maxSize {
		lruElement := c.ll.Back()
		if lruElement != nil {
			c.ll.Remove(lruElement)
			lruItem := lruElement.Value.(*item)
			delete(c.items, lruItem.Key)
		}
	}

	return nil
}

func (c *cacheShard) get(key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		it := elem.Value.(*item)

		// Пассивное вытеснение по TTL
		if it.Expiration > 0 && time.Now().UnixNano() > it.Expiration {
			c.ll.Remove(elem)
			delete(c.items, key)
			return nil, ErrNotFound
		}

		// Обновление LRU
		c.ll.MoveToFront(elem)
		return it.Value, nil
	}

	return nil, ErrNotFound
}

func (c *cacheShard) stop() {
	close(c.stopCh)
}

func (c *cacheShard) delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.ll.Remove(elem)
		delete(c.items, key)
	}

	return nil
}

// fnv.New32a() очень легковесен и создается моментально в каждой горутине, засчет этого можно не передавать hash и mutex
func (s *ShardedCache) getShard(key string) *cacheShard {
	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	hash := hasher.Sum32()

	sharedIndex := hash % s.shardCount
	return s.shards[sharedIndex]
}

func (s *ShardedCache) Set(key string, value interface{}, ttl time.Duration) error {
	var expiration int64
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	shard := s.getShard(key)
	return shard.set(key, value, expiration)
}

func (s *ShardedCache) Get(key string) (interface{}, error) {
	shard := s.getShard(key)
	return shard.get(key)
}

func (s *ShardedCache) Delete(key string) error {
	shard := s.getShard(key)
	return shard.delete(key)
}

func (s *ShardedCache) Stop() {
	for _, shard := range s.shards {
		shard.stop()
	}
}

func (s *ShardedCache) SaveToFile(path string) error {
	return nil
}

func (s *ShardedCache) LoadFromFile(path string) error {
	return nil
}
