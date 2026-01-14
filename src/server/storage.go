package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"
	
)

type TimeEvent struct {
    expiryTime time.Time
    timeToLive time.Duration
}

type TTLMap struct {
    mu   sync.RWMutex
    data map[string]TimeEvent
}


type Store struct {
	mu   sync.RWMutex
	data map[string]interface{}
	volatileKeyMap TTLMap
}

func newStore() *Store {
	return &Store{data: make(map[string]interface{}), volatileKeyMap: TTLMap{data: make(map[string]TimeEvent)}}
}

func WrapValue(val interface{}) ([]byte, bool) {
	switch v := val.(type) {
	case []byte:
		return v, true
	case int64:
		return []byte(strconv.FormatInt(v, 10)), true
	default:
		return nil, false
	}
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, exists := s.data[key]
	if !exists {
		return nil, false
	}
	switch s.isVolatile(key){
	case true:
		if s.volatileKeyMap.IsValid(key){
			res, wrapped := WrapValue(val)
			return res, wrapped
		} else {
			return nil, false
		}
	case false:
		res, wrapped := WrapValue(val)
		return res, wrapped
	}
	return nil, false
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

func (s *Store) isVolatile(key string) bool {
	s.volatileKeyMap.mu.RLock()
	defer s.volatileKeyMap.mu.RUnlock()
	_, exists := s.volatileKeyMap.data[key]

	if !exists {
		return false
	}
	return true
}

func (m *TTLMap) Set(key string, ttl time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.data[key] = TimeEvent{
		expiryTime: time.Now().Add(ttl),
		timeToLive: ttl,
	}
}

func (m *TTLMap) Delete(key string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    delete(m.data, key)
}

func (m *TTLMap) GetExpiry(key string) (time.Time, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    timeEvent, ok := m.data[key]
	if !ok {
        return time.Now(), fmt.Errorf("Key does not have a TTL or does not exist")
    }
	expiry := timeEvent.expiryTime
	return expiry, nil
}

func (m *TTLMap) GetDuration(key string) (time.Duration, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    timeEvent, ok := m.data[key]
	if !ok {
        return time.Duration(0), fmt.Errorf("Key does not have a TTL or does not exist")
    }
	Duration := timeEvent.timeToLive
	return Duration, nil
}

func (m *TTLMap) IsValid(key string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()

    TimeEvent, ok := m.data[key]
    if !ok {
        return false
    }
	expiry := TimeEvent.expiryTime

    if time.Now().After(expiry) {
        delete(m.data, key)
        return false
    }

    return true
}


