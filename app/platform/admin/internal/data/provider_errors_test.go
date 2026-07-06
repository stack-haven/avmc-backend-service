package data

import (
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

func TestProvidersReturnConfigErrors(t *testing.T) {
	logger := log.NewStdLogger(testWriter{})
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "ent client",
			run: func() error {
				_, err := NewEntClient(nil, logger)
				return err
			},
		},
		{
			name: "ent data",
			run: func() error {
				_, err := NewEntData(nil, logger)
				return err
			},
		},
		{
			name: "redis client",
			run: func() error {
				_, err := NewRedisClient(nil, logger)
				return err
			},
		},
		{
			name: "authenticator",
			run: func() error {
				_, err := NewAuthenticator(nil, logger, nil)
				return err
			},
		},
		{
			name: "authorizer",
			run: func() error {
				_, err := NewAuthorizer(nil, nil, logger)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("error = nil")
			}
		})
	}
}

type testWriter struct{}

func (testWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
