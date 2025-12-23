package main

import (
	"sync"
	"testing"
)

func TestStoreSetGet(t *testing.T) {
	store := newStore()
	store.Set("key1", []byte("value1"))

	val, exists := store.Get("key1")
	if !exists {
		t.Fatal("expected key to exist")
	}
	if string(val) != "value1" {
		t.Errorf("expected 'value1', got '%s'", val)
	}
}

func TestStoreGetMissing(t *testing.T) {
	store := newStore()
	_, exists := store.Get("nonexistent")
	if exists {
		t.Error("expected key to not exist")
	}
}

func TestStoreDelSingle(t *testing.T) {
	store := newStore()
	store.Set("key1", []byte("value1"))

	count := store.Del("key1")
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	_, exists := store.Get("key1")
	if exists {
		t.Error("expected key to be deleted")
	}
}

func TestStoreDelMultiple(t *testing.T) {
	store := newStore()
	store.Set("key1", []byte("val1"))
	store.Set("key2", []byte("val2"))
	store.Set("key3", []byte("val3"))

	count := store.Del("key1", "key2", "nonexistent")
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestStoreDelNonexistent(t *testing.T) {
	store := newStore()
	count := store.Del("nonexistent")
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestStoreOverwrite(t *testing.T) {
	store := newStore()
	store.Set("key1", []byte("value1"))
	store.Set("key1", []byte("value2"))

	val, _ := store.Get("key1")
	if string(val) != "value2" {
		t.Errorf("expected 'value2', got '%s'", val)
	}
}

func TestStoreConcurrentReads(t *testing.T) {
	store := newStore()
	store.Set("key1", []byte("value1"))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, exists := store.Get("key1")
			if !exists || string(val) != "value1" {
				t.Error("concurrent read failed")
			}
		}()
	}
	wg.Wait()
}

func TestStoreConcurrentWrites(t *testing.T) {
	store := newStore()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key"
			store.Set(key, []byte("value"))
		}(i)
	}
	wg.Wait()

	val, exists := store.Get("key")
	if !exists || string(val) != "value" {
		t.Error("concurrent write failed")
	}
}

func TestStoreConcurrentReadWrite(t *testing.T) {
	store := newStore()
	store.Set("key1", []byte("initial"))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			store.Set("key1", []byte("updated"))
		}()
		go func() {
			defer wg.Done()
			store.Get("key1")
		}()
	}
	wg.Wait()
}

func TestStoreConcurrentDeletes(t *testing.T) {
	store := newStore()
	for i := 0; i < 10; i++ {
		store.Set("key", []byte("value"))
	}

	var wg sync.WaitGroup
	totalDeleted := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := store.Del("key")
			mu.Lock()
			totalDeleted += count
			mu.Unlock()
		}()
	}
	wg.Wait()

	if totalDeleted > 1 {
		t.Errorf("expected at most 1 deletion, got %d", totalDeleted)
	}
}

func TestStoreIncr(t *testing.T) {
	store := newStore()
	val, err := store.Incr("counter")
	if err != nil || val != 1 {
		t.Errorf("expected 1, got %d, err: %v", val, err)
	}

	val, err = store.Incr("counter")
	if err != nil || val != 2 {
		t.Errorf("expected 2, got %d, err: %v", val, err)
	}
}

func TestStoreDecr(t *testing.T) {
	store := newStore()
	val, err := store.Decr("counter")
	if err != nil || val != -1 {
		t.Errorf("expected -1, got %d, err: %v", val, err)
	}

	val, err = store.Decr("counter")
	if err != nil || val != -2 {
		t.Errorf("expected -2, got %d, err: %v", val, err)
	}
}

func TestStoreIncrBy(t *testing.T) {
	store := newStore()
	val, err := store.IncrBy("counter", 5)
	if err != nil || val != 5 {
		t.Errorf("expected 5, got %d, err: %v", val, err)
	}

	val, err = store.IncrBy("counter", 10)
	if err != nil || val != 15 {
		t.Errorf("expected 15, got %d, err: %v", val, err)
	}

	val, err = store.IncrBy("counter", -3)
	if err != nil || val != 12 {
		t.Errorf("expected 12, got %d, err: %v", val, err)
	}
}

func TestStoreIncrStringValue(t *testing.T) {
	store := newStore()
	store.Set("num", []byte("10"))

	val, err := store.Incr("num")
	if err != nil || val != 11 {
		t.Errorf("expected 11, got %d, err: %v", val, err)
	}
}

func TestStoreIncrInvalidString(t *testing.T) {
	store := newStore()
	store.Set("key", []byte("notanumber"))

	_, err := store.Incr("key")
	if err == nil {
		t.Error("expected error for non-integer value")
	}
}

func TestStoreConcurrentIncr(t *testing.T) {
	store := newStore()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Incr("counter")
		}()
	}
	wg.Wait()

	val, _ := store.Get("counter")
	if string(val) != "100" {
		t.Errorf("expected '100', got '%s'", val)
	}
}
