package cache_test

import (
	"gocache/internal/cache"
	"path/filepath"
	"testing"
	"time"
)

func TestShardedCache_LRU_Mechanics(t *testing.T) {
	shardCount := uint32(1)
	maxSize := 3
	c := cache.NewShardedCache(shardCount, maxSize, 0)

	c.Set("A", 1, 0) // Список: [A]
	c.Set("B", 2, 0) // Список: [B, A]
	c.Set("C", 3, 0) // Список: [C, B, A]

	if _, err := c.Get("A"); err != nil {
		t.Fatalf("'A' should be in cache at this point, but got error: %v", err)
	}

	c.Set("D", 4, 0)

	if _, err := c.Get("B"); err == nil {
		t.Fatal("'B' should have been evicted, but it is still in cache")
	}

	if _, err := c.Get("A"); err != nil {
		t.Fatal("'A' should be in cache, but it was evicted")
	}
	if _, err := c.Get("C"); err != nil {
		t.Fatal("'C' should be in cache, but it was evicted")
	}
	if _, err := c.Get("D"); err != nil {
		t.Fatal("'D' should be in cache, but it was not found")
	}
}

func TestSharedCache_Persistence(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "cache.goc")
	cache1 := cache.NewShardedCache(32, 10, 10*time.Second)

	keyValues := map[string]string{
		"A": "A",
		"B": "B",
		"C": "C",
		"D": "D",
	}

	for k, v := range keyValues {
		cache1.Set(k, v, 0)
	}

	// Act 1 - вызвать SaveToFile и проверить на ошибку
	err := cache1.SaveToFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cache1.Stop()

	// Act 2 - загрузка из файла
	cache2 := cache.NewShardedCache(32, 10, 10*time.Second)
	err = cache2.LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	for k, v := range keyValues {
		retrievedValue, err := cache2.Get(k)
		if err != nil {
			t.Fatal(err)
		}
		if retrievedValue.(string) != v {
			t.Fatalf("retrieved value does not match stored value, expected %s, got %s", v, retrievedValue)
		}
	}
}

func TestShardedCache_IncrDecr(t *testing.T) {
	// Arrange
	shardedCache := cache.NewShardedCache(4, 100, 0)

	// Act 1 - incr
	val, err := shardedCache.Incr("A")
	if err != nil {
		t.Fatal(err)
	}

	// Assert 1
	if val != 1 {
		t.Fatalf("incr returned wrong value, expected 1, got %d", val)
	}

	val, err = shardedCache.Incr("A")
	if err != nil {
		t.Fatal(err)
	}
	if val != 2 {
		t.Fatalf("incr returned wrong value, expected 2, got %d", val)
	}

	// Act 2 - decr
	val, err = shardedCache.Decr("B")
	if err != nil {
		t.Fatal(err)
	}

	// Assert 2
	if val != -1 {
		t.Fatalf("incr returned wrong value, expected -1, got %d", val)
	}

	val, err = shardedCache.Decr("B")
	if err != nil {
		t.Fatal(err)
	}
	if val != -2 {
		t.Fatalf("incr returned wrong value, expected 1, got %d", val)
	}

	// Act 3
	shardedCache.Set("string_key", "string_value", 0)
	_, err = shardedCache.Incr("string_key")
	if err == nil {
		t.Fatal("should have errored")
	}

	_, err = shardedCache.Decr("string_key")
	if err == nil {
		t.Fatal("should have errored")
	}
}
