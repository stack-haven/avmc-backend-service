package idempotency

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidRequest = errors.New("idempotency: invalid request")
	ErrConflict       = errors.New("idempotency: key already used by different request")
)

type Status string

const (
	StatusCompleted Status = "completed"
)

type Request struct {
	Scope       string
	Key         string
	Fingerprint string
	TTL         time.Duration
}

type Record struct {
	Scope       string
	Key         string
	Fingerprint string
	Status      Status
	Value       []byte
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Result struct {
	Record Record
	Replay bool
}

type Store interface {
	Get(context.Context, string, string) (Record, bool, error)
	SaveCompleted(context.Context, Record) (Record, bool, error)
}

type Manager struct {
	store Store
	now   func() time.Time
}

func NewManager(store Store) *Manager {
	return &Manager{
		store: store,
		now:   time.Now,
	}
}

func (m *Manager) Execute(ctx context.Context, req Request, fn func(context.Context) ([]byte, error)) (Result, error) {
	if m == nil || m.store == nil || fn == nil {
		return Result{}, ErrInvalidRequest
	}
	req, err := normalizeRequest(req)
	if err != nil {
		return Result{}, err
	}
	if existing, ok, err := m.store.Get(ctx, req.Scope, req.Key); err != nil {
		return Result{}, err
	} else if ok {
		if existing.Fingerprint != req.Fingerprint {
			return Result{}, ErrConflict
		}
		return Result{Record: existing, Replay: true}, nil
	}

	value, err := fn(ctx)
	if err != nil {
		return Result{}, err
	}
	now := m.now().UTC()
	record := Record{
		Scope:       req.Scope,
		Key:         req.Key,
		Fingerprint: req.Fingerprint,
		Status:      StatusCompleted,
		Value:       append([]byte(nil), value...),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if req.TTL > 0 {
		record.ExpiresAt = now.Add(req.TTL)
	}
	saved, created, err := m.store.SaveCompleted(ctx, record)
	if err != nil {
		return Result{}, err
	}
	if !created {
		if saved.Fingerprint != req.Fingerprint {
			return Result{}, ErrConflict
		}
		return Result{Record: saved, Replay: true}, nil
	}
	return Result{Record: saved}, nil
}

func normalizeRequest(req Request) (Request, error) {
	req.Scope = strings.TrimSpace(req.Scope)
	req.Key = strings.TrimSpace(req.Key)
	req.Fingerprint = strings.TrimSpace(req.Fingerprint)
	if req.Scope == "" || req.Key == "" || req.Fingerprint == "" {
		return Request{}, ErrInvalidRequest
	}
	if req.TTL < 0 {
		return Request{}, ErrInvalidRequest
	}
	return req, nil
}
