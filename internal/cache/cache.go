package cache

import (
	"container/list"
	"encoding/gob"
	"errors"
	"hash/fnv"
	_ "hash/fnv"
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
	Incr(key string) (int64, error)
	Decr(key string) (int64, error)
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

func NewShardedCache(shardCount uint32, totalMaxSize int, cleanupInterval time.Duration) *ShardedCache {
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

		if cleanupInterval > 0 {
			go sc.shards[i].cleanupLoop(cleanupInterval)
		}
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

func (c *cacheShard) incr(key string, delta int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if ok {
		c.ll.MoveToFront(elem)
		it := elem.Value.(*item)

		var newValue int64
		switch v := it.Value.(type) {
		case int64:
			newValue = int64(v) + delta
			it.Value = newValue
		case int:
			newValue = int64(v) + delta
			it.Value = newValue
		case int32:
			newValue = int64(v) + delta
			it.Value = newValue
		case string:
			return 0, errors.New("value is not an integer")
		default:
			return 0, errors.New("value is not a supported number type")
		}

		return newValue, nil

	} else {
		it := &item{Key: key, Value: delta, Expiration: 0}
		elem := c.ll.PushFront(it)

		c.items[key] = elem

		if c.maxSize > 0 && c.ll.Len() > c.maxSize {
			lruElement := c.ll.Back()
			if lruElement != nil {
				c.ll.Remove(lruElement)
				lruItem := lruElement.Value.(*item)
				delete(c.items, lruItem.Key)
			}
		}
		return delta, nil
	}
}

func (c *cacheShard) set(key string, value interface{}, expiration int64) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		it := elem.Value.(*item)
		it.Value = value
		it.Expiration = expiration
	} else {
		it := &item{Key: key, Value: value, Expiration: expiration}
		elem := c.ll.PushFront(it)
		c.items[key] = elem
	}

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

		if it.Expiration > 0 && time.Now().UnixNano() > it.Expiration {
			c.ll.Remove(elem)
			delete(c.items, key)
			return nil, ErrNotFound
		}

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

func (sc *ShardedCache) Incr(key string) (int64, error) {
	shard := sc.getShard(key)
	return shard.incr(key, 1)
}

func (sc *ShardedCache) Decr(key string) (int64, error) {
	shard := sc.getShard(key)
	return shard.incr(key, -1)
}

// fnv.New32a() очень легковесен и создается моментально в каждой горутине, засчет этого можно не передавать hash и mutex
func (sc *ShardedCache) getShard(key string) *cacheShard {
	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	hash := hasher.Sum32()

	sharedIndex := hash % sc.shardCount
	return sc.shards[sharedIndex]
}

func (sc *ShardedCache) Set(key string, value interface{}, ttl time.Duration) error {
	var expiration int64
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	shard := sc.getShard(key)
	return shard.set(key, value, expiration)
}

func (sc *ShardedCache) Get(key string) (interface{}, error) {
	shard := sc.getShard(key)
	return shard.get(key)
}

func (sc *ShardedCache) Delete(key string) error {
	shard := sc.getShard(key)
	return shard.delete(key)
}

func (sc *ShardedCache) Stop() {
	for _, shard := range sc.shards {
		shard.stop()
	}
}

func (sc *ShardedCache) SaveToFile(path string) error {
	itemsToSave := make(map[string]*item)

	for _, shard := range sc.shards {
		shard.mu.RLock()
		for _, elem := range shard.items {
			it := elem.Value.(*item)
			itemsToSave[it.Key] = it
		}
		shard.mu.RUnlock()
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

func (sc *ShardedCache) LoadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)

	var itemsToLoad map[string]*item
	err = decoder.Decode(&itemsToLoad)
	if err != nil {
		return err
	}

	for _, it := range itemsToLoad {
		key := it.Key
		shard := sc.getShard(key)
		shard.mu.Lock()

		elem := shard.ll.PushFront(it)
		shard.items[key] = elem

		shard.mu.Unlock()
	}

	return nil
}
