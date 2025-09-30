package cache

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCache_SetGetDelete(t *testing.T) {
	c := New(1 * time.Second)
	key := "key"
	value := "value"

	_, err := c.Get(key) // Ключа еще нет
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}

	err = c.Set(key, value, 0)
	if err != nil {
		t.Errorf("got error %v, want %v", err, nil)
	}

	existsValue, err := c.Get(key) // Получаем установленное значение
	if err != nil {
		t.Fatalf("Expected no error, but got %v", err)
	}

	if existsValue != value {
		t.Errorf("got %v, want %v", existsValue, value)
	}

	err = c.Delete(key) // Удаляем
	if err != nil {
		t.Errorf("got error %v, want %v", err, nil)
	}

	_, err = c.Get(key) // Получаем удаленное
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected error %v after delete, but got %v", ErrNotFound, err)
	}
}

func TestCache_Concurrency(t *testing.T) {
	c := New(10 * time.Second)
	numGoroutines := 100
	iterationsPerGoroutine := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			key := "key"
			value := "value"

			for j := 0; j < iterationsPerGoroutine; j++ {
				err := c.Set(key, value, -1)
				if err != nil {
					t.Errorf("unexpected error on Set: %v", err)
				}

				val, err := c.Get(key)

				if err != nil && !errors.Is(err, ErrNotFound) {
					t.Errorf("unexpected error on Get: %v", err)
				}

				if err == nil && val != value {
					t.Errorf("got wrong value: want %q, got %q", value, val)
				}

				err = c.Delete(key)
				if err != nil {
					t.Errorf("unexpected error on Delete: %v", err)
				}
			}
		}()
	}

	// Assert: Ждем завершения всех горутин.
	wg.Wait()
}

func TestCache_TTL(t *testing.T) {
	c := New(1 * time.Second)
	key := "key"
	value := "value"
	err := c.Set(key, value, 50*time.Millisecond)
	if err != nil {
		t.Errorf("got error %v, want %v", err, nil)
	}
	val, err := c.Get(key)
	if err != nil {
		t.Errorf("got error %v, want %v", err, nil)
	}
	if val != value {
		t.Errorf("got %v, want %v", val, value)
	}
	time.Sleep(50 * time.Millisecond)
	val, err = c.Get(key)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}
}
