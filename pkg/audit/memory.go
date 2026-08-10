package audit

import (
	"context"
	"sync"
)

// MemoryClient stores audit records in memory. Thread-safe.
type MemoryClient struct {
	mu      sync.Mutex
	Records []*Record
}

func NewMemoryClient() *MemoryClient {
	return &MemoryClient{}
}

func (c *MemoryClient) Append(_ context.Context, record *Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Records = append(c.Records, record)
	return nil
}

func (c *MemoryClient) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.Records)
}
