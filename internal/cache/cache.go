package cache

import (
	"errors"
	"hash"
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
	hash       hash.Hash32
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
		hash:       fnv.New32a(),
	}

	for i := 0; i < int(shardCount); i++ {
		sc.shards[i] = &cacheShard{
			items: make(map[string]*item),
		}
	}
	return sc
}

func (s *ShardedCache) getShard(key string) *cacheShard {
	s.hash.Write([]byte(key))
	hashKey := s.hash.Sum32()
	s.hash.Reset()

	shardIndex := hashKey % s.shardCount
	return s.shards[shardIndex]
}

// test
