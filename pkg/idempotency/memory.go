package idempotency

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.Mutex
	records map[string]Record
	now     func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]Record),
		now:     time.Now,
	}
}

func (s *MemoryStore) Get(_ context.Context, scope string, key string) (Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[recordKey(scope, key)]
	if !ok {
		return Record{}, false, nil
	}
	if recordExpired(record, s.now()) {
		delete(s.records, recordKey(scope, key))
		return Record{}, false, nil
	}
	record.Value = append([]byte(nil), record.Value...)
	return record, true, nil
}

func (s *MemoryStore) SaveCompleted(_ context.Context, record Record) (Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := recordKey(record.Scope, record.Key)
	if existing, ok := s.records[key]; ok {
		if recordExpired(existing, s.now()) {
			delete(s.records, key)
		} else {
			existing.Value = append([]byte(nil), existing.Value...)
			return existing, false, nil
		}
	}
	record.Value = append([]byte(nil), record.Value...)
	s.records[key] = record
	return record, true, nil
}

func recordKey(scope string, key string) string {
	return scope + "\x00" + key
}

func recordExpired(record Record, now time.Time) bool {
	return !record.ExpiresAt.IsZero() && !now.Before(record.ExpiresAt)
}
