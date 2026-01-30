package main

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"
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

func TestIsVolatile(t *testing.T) {
	store := &Store{}
	store.volatileKeyMap.data = make(map[string]ExpirationTime)

	// key does not exist
	if store.isVolatile("missing") {
		t.Fatalf("expected false for non-volatile key")
	}

	// add key
	store.volatileKeyMap.data["volatile"] = ExpirationTime{expiryTime: time.Now(), durationSet: time.Duration(time.Minute * 5)}

	if !store.isVolatile("volatile") {
		t.Fatalf("expected true for volatile key")
	}
}

func newTTLMap() *TTLMap {
	return &TTLMap{
		data: make(map[string]ExpirationTime),
	}
}

func TestTTLMapGetExpiry(t *testing.T) {
	m := newTTLMap()
	ttl := 100 * time.Millisecond

	m.Set("key", ttl)

	expiry, err := m.GetSetExpiry("key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expiry.Before(time.Now()) {
		t.Fatalf("expiry should be in the future")
	}
}

func TestTTLMapGetExpiryMissingKey(t *testing.T) {
	m := newTTLMap()

	_, err := m.GetSetExpiry("missing")
	if err == nil {
		t.Fatalf("expected error for missing key")
	}
}

func TestTTLMapGetDuration(t *testing.T) {
	m := newTTLMap()
	ttl := 200 * time.Millisecond

	m.Set("key", ttl)

	d, err := m.GetSetDuration("key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d != ttl {
		t.Fatalf("expected duration %v, got %v", ttl, d)
	}
}

func TestTTLMapGetDurationMissingKey(t *testing.T) {
	m := newTTLMap()

	_, err := m.GetSetDuration("missing")
	if err == nil {
		t.Fatalf("expected error for missing key")
	}
}

func TestTTLMapIsValidTrue(t *testing.T) {
	m := newTTLMap()
	m.Set("key", time.Second)

	if !m.IsValid("key") {
		t.Fatalf("expected key to be valid")
	}
}

func TestTTLMapIsValidExpired(t *testing.T) {
	m := newTTLMap()
	m.Set("key", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	if m.IsValid("key") {
		t.Fatalf("expected key to be expired")
	}

	// ensure expired key is deleted
	if _, ok := m.data["key"]; ok {
		t.Fatalf("expected expired key to be removed")
	}
}

func TestTTLMapIsValidMissingKey(t *testing.T) {
	m := newTTLMap()

	if m.IsValid("missing") {
		t.Fatalf("expected false for missing key")
	}
}

func TestIsVolatileRace(t *testing.T) {
	store := &Store{}
	store.volatileKeyMap.data = make(map[string]ExpirationTime)

	var wg sync.WaitGroup

	// writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			for j := 0; j < 1000; j++ {
				store.volatileKeyMap.mu.Lock()
				store.volatileKeyMap.data[key] = ExpirationTime{expiryTime: time.Now().Add(time.Minute), durationSet: time.Minute}
				store.volatileKeyMap.mu.Unlock()
			}
		}(i)
	}

	// readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			for j := 0; j < 1000; j++ {
				_ = store.isVolatile(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestTTLMapConcurrentAccessRace(t *testing.T) {
	m := newTTLMap()
	var wg sync.WaitGroup

	// writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			for j := 0; j < 1000; j++ {
				m.Set(key, time.Second)
			}
		}(i)
	}

	// readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			for j := 0; j < 1000; j++ {
				m.GetSetExpiry(key)
				m.GetSetDuration(key)
			}
		}(i)
	}

	wg.Wait()
}

// ==================== List Storage Unit Tests ====================

func TestLPushSingle(t *testing.T) {
	store := newStore()
	n, err := store.LPush("mylist", []byte("a"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestLPushMultiple(t *testing.T) {
	store := newStore()
	n, err := store.LPush("mylist", []byte("a"), []byte("b"), []byte("c"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
	// LPUSH a b c => [c, b, a]
	result, err := store.LRange("mylist", 0, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"c", "b", "a"}
	for i, v := range result {
		if string(v) != expected[i] {
			t.Errorf("index %d: expected %q, got %q", i, expected[i], string(v))
		}
	}
}

func TestLPushCreatesKey(t *testing.T) {
	store := newStore()
	n, err := store.LPush("newkey", []byte("x"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestLPushWrongType(t *testing.T) {
	store := newStore()
	store.Set("str", []byte("hello"))
	_, err := store.LPush("str", []byte("x"))
	if err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
}

func TestRPushSingle(t *testing.T) {
	store := newStore()
	n, err := store.RPush("mylist", []byte("a"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestRPushMultiple(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"))
	n, err := store.RPush("mylist", []byte("b"), []byte("c"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
	result, err := store.LRange("mylist", 0, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"a", "b", "c"}
	for i, v := range result {
		if string(v) != expected[i] {
			t.Errorf("index %d: expected %q, got %q", i, expected[i], string(v))
		}
	}
}

func TestRPushWrongType(t *testing.T) {
	store := newStore()
	store.Set("str", []byte("hello"))
	_, err := store.RPush("str", []byte("x"))
	if err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
}

func TestLPopSingle(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"), []byte("c"))
	result, err := store.LPop("mylist", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || string(result[0]) != "a" {
		t.Errorf("expected [a], got %v", result)
	}
}

func TestLPopCount(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"), []byte("c"))
	result, err := store.LPop("mylist", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(result))
	}
	if string(result[0]) != "a" || string(result[1]) != "b" {
		t.Errorf("expected [a, b], got %v", result)
	}
}

func TestLPopNonExistent(t *testing.T) {
	store := newStore()
	result, err := store.LPop("missing", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestLPopMoreThanAvailable(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"))
	result, err := store.LPop("mylist", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 elements, got %d", len(result))
	}
}

func TestLPopAutoDelete(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"))
	store.LPop("mylist", 1)
	n, _ := store.LLen("mylist")
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
	// Key should be gone
	store.mu.RLock()
	_, exists := store.data["mylist"]
	store.mu.RUnlock()
	if exists {
		t.Error("expected key to be deleted")
	}
}

func TestLPopWrongType(t *testing.T) {
	store := newStore()
	store.Set("str", []byte("hello"))
	_, err := store.LPop("str", 1)
	if err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
}

func TestRPopSingle(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"), []byte("c"))
	result, err := store.RPop("mylist", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || string(result[0]) != "c" {
		t.Errorf("expected [c], got %v", result)
	}
}

func TestRPopCount(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"), []byte("c"))
	result, err := store.RPop("mylist", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(result))
	}
	if string(result[0]) != "b" || string(result[1]) != "c" {
		t.Errorf("expected [b, c], got %v", result)
	}
}

func TestRPopNonExistent(t *testing.T) {
	store := newStore()
	result, err := store.RPop("missing", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestRPopAutoDelete(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"))
	store.RPop("mylist", 1)
	store.mu.RLock()
	_, exists := store.data["mylist"]
	store.mu.RUnlock()
	if exists {
		t.Error("expected key to be deleted")
	}
}

func TestRPopWrongType(t *testing.T) {
	store := newStore()
	store.Set("str", []byte("hello"))
	_, err := store.RPop("str", 1)
	if err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
}

func TestLRangeBasic(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"), []byte("c"), []byte("d"))
	result, err := store.LRange("mylist", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || string(result[0]) != "b" || string(result[1]) != "c" {
		t.Errorf("expected [b, c], got %v", result)
	}
}

func TestLRangeNegativeIndices(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"), []byte("c"))
	result, err := store.LRange("mylist", -2, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || string(result[0]) != "b" || string(result[1]) != "c" {
		t.Errorf("expected [b, c], got %v", result)
	}
}

func TestLRangeOutOfBounds(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"))
	result, err := store.LRange("mylist", 0, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 elements, got %d", len(result))
	}
}

func TestLRangeEmptyRange(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"))
	result, err := store.LRange("mylist", 5, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestLRangeNonExistent(t *testing.T) {
	store := newStore()
	result, err := store.LRange("missing", 0, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestLRangeWrongType(t *testing.T) {
	store := newStore()
	store.Set("str", []byte("hello"))
	_, err := store.LRange("str", 0, -1)
	if err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
}

func TestLLenExisting(t *testing.T) {
	store := newStore()
	store.RPush("mylist", []byte("a"), []byte("b"), []byte("c"))
	n, err := store.LLen("mylist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestLLenNonExistent(t *testing.T) {
	store := newStore()
	n, err := store.LLen("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestLLenWrongType(t *testing.T) {
	store := newStore()
	store.Set("str", []byte("hello"))
	_, err := store.LLen("str")
	if err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
}

// Cross-type WRONGTYPE tests
func TestGetOnListKey(t *testing.T) {
	store := newStore()
	store.LPush("mylist", []byte("a"))
	_, _, err := store.GetWithTypeCheck("mylist")
	if err == nil {
		t.Fatal("expected WRONGTYPE error for GET on list key")
	}
}

func TestIncrOnListKey(t *testing.T) {
	store := newStore()
	store.LPush("mylist", []byte("a"))
	_, err := store.Incr("mylist")
	if err == nil {
		t.Fatal("expected WRONGTYPE error for INCR on list key")
	}
}

func TestDecrOnListKey(t *testing.T) {
	store := newStore()
	store.LPush("mylist", []byte("a"))
	_, err := store.Decr("mylist")
	if err == nil {
		t.Fatal("expected WRONGTYPE error for DECR on list key")
	}
}

func TestConcurrentListPushes(t *testing.T) {
	store := newStore()
	var wg sync.WaitGroup
	pushesPerGoroutine := 100
	goroutines := 10
	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < pushesPerGoroutine; j++ {
				store.LPush("mylist", []byte("x"))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < pushesPerGoroutine; j++ {
				store.RPush("mylist", []byte("y"))
			}
		}()
	}
	wg.Wait()
	n, _ := store.LLen("mylist")
	expected := int64(goroutines * pushesPerGoroutine * 2)
	if n != expected {
		t.Errorf("expected %d, got %d", expected, n)
	}
}

// ==================== Property-Based Tests ====================

// byteSliceGen generates a random []byte element
func byteSliceGen() *rapid.Generator[[]byte] {
	return rapid.Custom(func(t *rapid.T) []byte {
		return []byte(rapid.StringN(1, 1, 50).Draw(t, "elem"))
	})
}

// byteSliceListGen generates a random [][]byte list
func byteSliceListGen(minLen, maxLen int) *rapid.Generator[[][]byte] {
	return rapid.Custom(func(t *rapid.T) [][]byte {
		n := rapid.IntRange(minLen, maxLen).Draw(t, "len")
		list := make([][]byte, n)
		for i := 0; i < n; i++ {
			list[i] = byteSliceGen().Draw(t, fmt.Sprintf("elem%d", i))
		}
		return list
	})
}

// Feature: redis-list-operations, Property 1: LPUSH length and ordering invariant
func TestPropertyLPushLengthAndOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		initial := byteSliceListGen(0, 20).Draw(t, "initial")
		elements := byteSliceListGen(1, 10).Draw(t, "elements")

		store := newStore()
		// Seed with initial list via RPush
		if len(initial) > 0 {
			store.RPush("key", initial...)
		}

		n, err := store.LPush("key", elements...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedLen := int64(len(initial) + len(elements))
		if n != expectedLen {
			t.Fatalf("expected length %d, got %d", expectedLen, n)
		}

		result, _ := store.LRange("key", 0, -1)
		// Verify: reversed elements at head, then initial
		for i := 0; i < len(elements); i++ {
			if !bytes.Equal(result[i], elements[len(elements)-1-i]) {
				t.Fatalf("ordering mismatch at index %d", i)
			}
		}
		for i := 0; i < len(initial); i++ {
			if !bytes.Equal(result[len(elements)+i], initial[i]) {
				t.Fatalf("initial element mismatch at index %d", i)
			}
		}
	})
}

// Feature: redis-list-operations, Property 2: RPUSH length and ordering invariant
func TestPropertyRPushLengthAndOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		initial := byteSliceListGen(0, 20).Draw(t, "initial")
		elements := byteSliceListGen(1, 10).Draw(t, "elements")

		store := newStore()
		if len(initial) > 0 {
			store.RPush("key", initial...)
		}

		n, err := store.RPush("key", elements...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedLen := int64(len(initial) + len(elements))
		if n != expectedLen {
			t.Fatalf("expected length %d, got %d", expectedLen, n)
		}

		result, _ := store.LRange("key", 0, -1)
		// Verify: initial then elements in argument order
		for i := 0; i < len(initial); i++ {
			if !bytes.Equal(result[i], initial[i]) {
				t.Fatalf("initial element mismatch at index %d", i)
			}
		}
		for i := 0; i < len(elements); i++ {
			if !bytes.Equal(result[len(initial)+i], elements[i]) {
				t.Fatalf("element mismatch at index %d", i)
			}
		}
	})
}

// Feature: redis-list-operations, Property 3: LPOP returns head elements and shrinks list
func TestPropertyLPopHeadElements(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		list := byteSliceListGen(1, 20).Draw(t, "list")
		count := rapid.IntRange(1, len(list)+5).Draw(t, "count")

		store := newStore()
		store.RPush("key", list...)

		result, err := store.LPop("key", count)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		actualCount := count
		if actualCount > len(list) {
			actualCount = len(list)
		}

		if len(result) != actualCount {
			t.Fatalf("expected %d elements, got %d", actualCount, len(result))
		}

		for i := 0; i < actualCount; i++ {
			if !bytes.Equal(result[i], list[i]) {
				t.Fatalf("element mismatch at index %d", i)
			}
		}

		remaining, _ := store.LRange("key", 0, -1)
		expectedRemaining := list[actualCount:]
		if len(remaining) != len(expectedRemaining) {
			t.Fatalf("remaining length mismatch: expected %d, got %d", len(expectedRemaining), len(remaining))
		}
	})
}

// Feature: redis-list-operations, Property 4: RPOP returns tail elements and shrinks list
func TestPropertyRPopTailElements(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		list := byteSliceListGen(1, 20).Draw(t, "list")
		count := rapid.IntRange(1, len(list)+5).Draw(t, "count")

		store := newStore()
		store.RPush("key", list...)

		result, err := store.RPop("key", count)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		actualCount := count
		if actualCount > len(list) {
			actualCount = len(list)
		}

		if len(result) != actualCount {
			t.Fatalf("expected %d elements, got %d", actualCount, len(result))
		}

		// RPop returns elements from tail: list[len-count:]
		start := len(list) - actualCount
		for i := 0; i < actualCount; i++ {
			if !bytes.Equal(result[i], list[start+i]) {
				t.Fatalf("element mismatch at index %d", i)
			}
		}

		remaining, _ := store.LRange("key", 0, -1)
		expectedRemaining := list[:start]
		if len(remaining) != len(expectedRemaining) {
			t.Fatalf("remaining length mismatch: expected %d, got %d", len(expectedRemaining), len(remaining))
		}
	})
}

// Feature: redis-list-operations, Property 5: Pop empties list then deletes key
func TestPropertyPopDeletesKey(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		list := byteSliceListGen(1, 20).Draw(t, "list")
		useLPop := rapid.Bool().Draw(t, "useLPop")

		store := newStore()
		store.RPush("key", list...)

		if useLPop {
			store.LPop("key", len(list))
		} else {
			store.RPop("key", len(list))
		}

		store.mu.RLock()
		_, exists := store.data["key"]
		store.mu.RUnlock()
		if exists {
			t.Fatal("expected key to be deleted after popping all elements")
		}
	})
}

// Feature: redis-list-operations, Property 6: LRANGE returns correct slice with index normalization
func TestPropertyLRangeSlice(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		list := byteSliceListGen(1, 20).Draw(t, "list")
		length := len(list)
		start := rapid.IntRange(-length-5, length+5).Draw(t, "start")
		stop := rapid.IntRange(-length-5, length+5).Draw(t, "stop")

		store := newStore()
		store.RPush("key", list...)

		result, err := store.LRange("key", start, stop)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Compute expected using same normalization
		s := start
		e := stop
		if s < 0 {
			s = length + s
		}
		if e < 0 {
			e = length + e
		}
		if s < 0 {
			s = 0
		}
		if e >= length {
			e = length - 1
		}

		if s > e {
			if len(result) != 0 {
				t.Fatalf("expected empty result, got %d elements", len(result))
			}
			return
		}

		expected := list[s : e+1]
		if len(result) != len(expected) {
			t.Fatalf("length mismatch: expected %d, got %d", len(expected), len(result))
		}
		for i := range expected {
			if !bytes.Equal(result[i], expected[i]) {
				t.Fatalf("element mismatch at index %d", i)
			}
		}
	})
}

// Feature: redis-list-operations, Property 7: LLEN matches actual list length
func TestPropertyLLenMatchesLength(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		list := byteSliceListGen(0, 20).Draw(t, "list")

		store := newStore()
		if len(list) > 0 {
			store.RPush("key", list...)
		}

		n, err := store.LLen("key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != int64(len(list)) {
			t.Fatalf("expected %d, got %d", len(list), n)
		}
	})
}

// Feature: redis-list-operations, Property 8: WRONGTYPE on list operations against non-list keys
func TestPropertyWrongTypeOnNonListKeys(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		val := byteSliceGen().Draw(t, "val")

		store := newStore()
		store.Set("key", val)

		if _, err := store.LPush("key", []byte("x")); err == nil {
			t.Fatal("expected WRONGTYPE for LPush")
		}
		if _, err := store.RPush("key", []byte("x")); err == nil {
			t.Fatal("expected WRONGTYPE for RPush")
		}
		if _, err := store.LPop("key", 1); err == nil {
			t.Fatal("expected WRONGTYPE for LPop")
		}
		if _, err := store.RPop("key", 1); err == nil {
			t.Fatal("expected WRONGTYPE for RPop")
		}
		if _, err := store.LRange("key", 0, -1); err == nil {
			t.Fatal("expected WRONGTYPE for LRange")
		}
		if _, err := store.LLen("key"); err == nil {
			t.Fatal("expected WRONGTYPE for LLen")
		}
	})
}

// Feature: redis-list-operations, Property 9: WRONGTYPE on string operations against list keys
func TestPropertyWrongTypeOnListKeys(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		list := byteSliceListGen(1, 10).Draw(t, "list")

		store := newStore()
		store.RPush("key", list...)

		if _, _, err := store.GetWithTypeCheck("key"); err == nil {
			t.Fatal("expected WRONGTYPE for Get on list key")
		}
		if _, err := store.Incr("key"); err == nil {
			t.Fatal("expected WRONGTYPE for Incr on list key")
		}
		if _, err := store.Decr("key"); err == nil {
			t.Fatal("expected WRONGTYPE for Decr on list key")
		}
	})
}

// Feature: redis-list-operations, Property 10: Concurrent pushes preserve total element count
func TestPropertyConcurrentPushesPreserveCount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numGoroutines := rapid.IntRange(2, 10).Draw(t, "goroutines")
		pushesPerGoroutine := rapid.IntRange(10, 50).Draw(t, "pushes")

		store := newStore()
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			useLPush := i%2 == 0
			go func() {
				defer wg.Done()
				for j := 0; j < pushesPerGoroutine; j++ {
					if useLPush {
						store.LPush("key", []byte("x"))
					} else {
						store.RPush("key", []byte("x"))
					}
				}
			}()
		}
		wg.Wait()

		n, _ := store.LLen("key")
		expected := int64(numGoroutines * pushesPerGoroutine)
		if n != expected {
			t.Fatalf("expected %d, got %d", expected, n)
		}
	})
}

// Feature: redis-list-operations, Property 11: LPUSH then LPOP round trip
func TestPropertyLPushLPopRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		elements := byteSliceListGen(1, 20).Draw(t, "elements")

		store := newStore()
		store.LPush("key", elements...)

		result, err := store.LPop("key", len(elements))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != len(elements) {
			t.Fatalf("expected %d elements, got %d", len(elements), len(result))
		}

		// LPUSH reverses, LPOP from head returns in reverse order of args
		// LPUSH a,b,c => [c,b,a], LPOP 3 => [c,b,a]
		// So result[i] == elements[len-1-i]
		for i := 0; i < len(elements); i++ {
			if !bytes.Equal(result[i], elements[len(elements)-1-i]) {
				t.Fatalf("round trip mismatch at index %d: expected %q, got %q",
					i, elements[len(elements)-1-i], result[i])
			}
		}
	})
}

// Feature: redis-list-operations, Property 12: RPUSH then RPOP round trip
func TestPropertyRPushRPopRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		elements := byteSliceListGen(1, 20).Draw(t, "elements")

		store := newStore()
		store.RPush("key", elements...)

		result, err := store.RPop("key", len(elements))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != len(elements) {
			t.Fatalf("expected %d elements, got %d", len(elements), len(result))
		}

		// RPUSH a,b,c => [a,b,c], RPOP 3 => [a,b,c]
		// So result[i] == elements[i]
		for i := 0; i < len(elements); i++ {
			if !bytes.Equal(result[i], elements[i]) {
				t.Fatalf("round trip mismatch at index %d: expected %q, got %q",
					i, elements[i], result[i])
			}
		}
	})
}
