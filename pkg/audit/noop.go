package audit

import "context"

// NoopClient is an audit client that discards all entries.
type NoopClient struct{}

func (NoopClient) Append(_ context.Context, _ *Record) error { return nil }
