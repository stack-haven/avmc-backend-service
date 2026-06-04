package safelogging

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

type secretRequest struct {
	Password string
}

func TestServerDoesNotLogRequestBody(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := log.NewStdLogger(&output)
	handler := Server(logger)(func(context.Context, any) (any, error) {
		return nil, nil
	})

	if _, err := handler(context.Background(), secretRequest{Password: "plain-text-secret"}); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if strings.Contains(output.String(), "plain-text-secret") || strings.Contains(output.String(), "Password") {
		t.Fatalf("log contains request body: %s", output.String())
	}
}

func TestServerLogsInternalErrorWithoutRequestBody(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := log.NewStdLogger(&output)
	handler := Server(logger)(func(context.Context, any) (any, error) {
		return nil, errors.New("database unavailable")
	})

	_, _ = handler(context.Background(), secretRequest{Password: "plain-text-secret"})
	if strings.Contains(output.String(), "plain-text-secret") {
		t.Fatalf("log contains request body: %s", output.String())
	}
	if !strings.Contains(output.String(), "database unavailable") {
		t.Fatalf("log does not contain internal error: %s", output.String())
	}
}
