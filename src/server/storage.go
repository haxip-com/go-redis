package main

import "sync"

type Store struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newStore() *Store {
	return &Store{data: make(map[string][]byte)}
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, exists := s.data[key]
	return val, exists
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
