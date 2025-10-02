package cache

import (
	"errors"
	"hash/fnv"
	_ "hash/fnv"
	"sync"
	"time"
)

const defaultShardCount = 32

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
	mu    *sync.Mutex
	items map[string]*item
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

func NewShardedCache(shardCount uint32) *ShardedCache {
	sc := &ShardedCache{
		shards:     make([]*cacheShard, shardCount),
		shardCount: shardCount,
	}

	for i := 0; i < int(shardCount); i++ {
		sc.shards[i] = &cacheShard{
			items: make(map[string]*item),
			mu:    new(sync.Mutex),
		}
	}
	return sc
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
	shard := s.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.items[key] = &item{Value: value, Expiration: time.Now().Add(ttl).Unix()}

	return nil
}
