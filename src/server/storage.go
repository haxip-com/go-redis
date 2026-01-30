package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"
	
)

type ExpirationTime struct {
    expiryTime time.Time
    durationSet time.Duration
}

type expirationSetter func(key string)

type TTLMap struct {
    mu   sync.RWMutex
    data map[string]ExpirationTime
}


type Store struct {
	mu   sync.RWMutex
	data map[string]interface{}
	volatileKeyMap TTLMap
}

func newStore() *Store {
	return &Store{data: make(map[string]interface{}), volatileKeyMap: TTLMap{data: make(map[string]ExpirationTime)}}
}

func WrapValue(val interface{}) ([]byte, bool) {
	switch v := val.(type) {
	case []byte:
		return v, true
	case int64:
		return []byte(strconv.FormatInt(v, 10)), true
	case [][]byte:
		return nil, false
	default:
		return nil, false
	}
}

var errWrongType = fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	val, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return nil, false
	}
	if _, isList := val.([][]byte); isList {
		return nil, false
	}
	// Only delete after RLock released
	if s.isVolatile(key) && !s.volatileKeyMap.IsValid(key) {
		defer func() {
			s.Del(key)
		}()
		return nil, false
	}
	return WrapValue(val)
}

func (s *Store) GetWithTypeCheck(key string) ([]byte, bool, error) {
	s.mu.RLock()
	val, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return nil, false, nil
	}
	if _, isList := val.([][]byte); isList {
		return nil, false, errWrongType
	}
	if s.isVolatile(key) && !s.volatileKeyMap.IsValid(key) {
		defer func() {
			s.Del(key)
		}()
		return nil, false, nil
	}
	b, ok := WrapValue(val)
	return b, ok, nil
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
	case [][]byte:
		return 0, errWrongType
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

func (m *TTLMap) setExpiration(key string, exp ExpirationTime) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = exp
}

func withUnixExpiry(m *TTLMap, t time.Time) (expirationSetter, time.Duration) {
	d := t.Sub(time.Now())
	return func(key string) {
		m.setExpiration(key, ExpirationTime{
			expiryTime:  t,
			durationSet: d,
		})
	}, d
}

func withTTL(m *TTLMap, ttl time.Duration) expirationSetter {
	return func(key string) {
		m.setExpiration(key, ExpirationTime{
			expiryTime:  time.Now().Add(ttl),
			durationSet: ttl,
		})
	}
}

func (m *TTLMap) Set(key string, ttl time.Duration) {
	m.setExpiration(key, ExpirationTime{
		expiryTime:  time.Now().Add(ttl),
		durationSet: ttl,
	})
}

func (m *TTLMap) apply(key string, set expirationSetter) {
	set(key)
}

func (m *TTLMap) Delete(key string) {
    m.mu.Lock()
    defer m.mu.Unlock()

	_, ok := m.data[key]

	if !ok {
        return
    }

    delete(m.data, key)
}

func (m *TTLMap) GetSetExpiry(key string) (time.Time, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    timeEvent, ok := m.data[key]
	if !ok {
        return time.Now(), fmt.Errorf("Key does not have a TTL or does not exist")
    }
	expiry := timeEvent.expiryTime
	return expiry, nil
}

func (m *TTLMap) GetTTL(key string) (time.Duration, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    timeEvent, ok := m.data[key]
	if !ok {
        return time.Duration(time.Microsecond), fmt.Errorf("Key does not have a TTL or does not exist")
    }
	expiry := timeEvent.expiryTime
	//expired key that was not deleted yet
	if !time.Now().Before(expiry){
		return time.Duration(time.Microsecond), fmt.Errorf("Key does not have a TTL or does not exist")
	}
	return expiry.Sub(time.Now()), nil
}

func (m *TTLMap) GetSetDuration(key string) (time.Duration, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    timeEvent, ok := m.data[key]
	if !ok {
        return time.Duration(0), fmt.Errorf("Key does not have a TTL or does not exist")
    }
	Duration := timeEvent.durationSet
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

    if !time.Now().Before(expiry) {
        delete(m.data, key)
        return false
    }

    return true
}

func (s *Store) getList(key string) ([][]byte, bool, error) {
	val, exists := s.data[key]
	if !exists {
		return nil, false, nil
	}
	switch v := val.(type) {
	case [][]byte:
		return v, true, nil
	default:
		return nil, false, fmt.Errorf("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
}

func (s *Store) LPush(key string, elements ...[]byte) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, _, err := s.getList(key)
	if err != nil {
		return 0, err
	}
	// Prepend: new = elements_reversed + existing
	newList := make([][]byte, 0, len(elements)+len(list))
	for i := len(elements) - 1; i >= 0; i-- {
		newList = append(newList, elements[i])
	}
	newList = append(newList, list...)
	s.data[key] = newList
	return int64(len(newList)), nil
}

func (s *Store) RPush(key string, elements ...[]byte) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, _, err := s.getList(key)
	if err != nil {
		return 0, err
	}
	list = append(list, elements...)
	s.data[key] = list
	return int64(len(list)), nil
}

func (s *Store) LPop(key string, count int) ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, exists, err := s.getList(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	if count > len(list) {
		count = len(list)
	}
	result := make([][]byte, count)
	copy(result, list[:count])
	list = list[count:]
	if len(list) == 0 {
		delete(s.data, key)
	} else {
		s.data[key] = list
	}
	return result, nil
}

func (s *Store) RPop(key string, count int) ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, exists, err := s.getList(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	if count > len(list) {
		count = len(list)
	}
	start := len(list) - count
	result := make([][]byte, count)
	copy(result, list[start:])
	list = list[:start]
	if len(list) == 0 {
		delete(s.data, key)
	} else {
		s.data[key] = list
	}
	return result, nil
}

func (s *Store) LRange(key string, start, stop int) ([][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list, exists, err := s.getList(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return [][]byte{}, nil
	}
	length := len(list)
	// Normalize negative indices
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}
	// Clamp
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		return [][]byte{}, nil
	}
	result := make([][]byte, stop-start+1)
	copy(result, list[start:stop+1])
	return result, nil
}

func (s *Store) LLen(key string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list, exists, err := s.getList(key)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	return int64(len(list)), nil
}

