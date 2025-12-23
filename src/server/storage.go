package main

import (
	"fmt"
	"strconv"
	"sync"
)

type Store struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

func newStore() *Store {
	return &Store{data: make(map[string]interface{})}
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, exists := s.data[key]
	if !exists {
		return nil, false
	}
	switch v := val.(type) {
	case []byte:
		return v, true
	case int64:
		return []byte(strconv.FormatInt(v, 10)), true
	default:
		return nil, false
	}
}

func (s *Store) Set(key string, val []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}

func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, key := range keys {
		if _, exists := s.data[key]; exists {
			delete(s.data, key)
			count++
		}
	}
	return count
}

func (s *Store) IncrBy(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.data[key]
	if !exists {
		s.data[key] = delta
		return delta, nil
	}

	switch v := val.(type) {
	case []byte:
		num64, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("ERR value is not an integer or out of range")
		}
		num64 += delta // Clear intent: add delta
		s.data[key] = num64
		return num64, nil
	case int64:
		v += delta // Clear intent: add delta
		s.data[key] = v
		return v, nil
	default:
		return 0, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
}

func (s *Store) Incr(key string) (int64, error) {
	return s.IncrBy(key, 1)
}

func (s *Store) Decr(key string) (int64, error) {
	return s.IncrBy(key, -1)
}
