package main

import "sync"

type Store struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newStore() *Store {
	return &Store{data: make(map[string][]byte)}
}

func (s *Store) Get(key string) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

func (s *Store) Set(key string, val []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}

func (s *Store) Del(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}
