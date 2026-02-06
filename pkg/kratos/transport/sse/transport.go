// Package sse implements a Server-Sent Events (SSE) transport for the Kratos framework.
//
// SSE is a server push technology that allows a server to send events to the client over HTTP.
// This package provides a Kratos-compatible transport implementation for SSE, enabling
// streaming data delivery for use cases like AI model responses, real-time updates, etc.
//
// Example usage:
//
// 1. Create a new SSE server:
//
//	srv := sse.NewServer(
//	    sse.WithAddress(":8080"),
//	    sse.WithPath("/events"),
//	    sse.WithMiddleware(middleware.Logger()),
//	)
//
// 2. Start the server:
//
//	if err := srv.Start(ctx); err != nil {
//	    log.Fatalf("failed to start server: %v", err)
//	}
//
// 3. Publish events to a stream:
//
//	streamID := "user-123"
//	event := sse.NewEvent()
//	event.Data = []byte(`{"message": "Hello, world!"}`)
//	srv.Publish(ctx, streamID, event)
//
// 4. Stop the server gracefully:
//
//	if err := srv.Stop(ctx); err != nil {
//	    log.Fatalf("failed to stop server: %v", err)
//	}
//
// The package also provides:
// - Stream management for handling multiple SSE connections
// - Middleware support for HTTP request processing
// - Health check endpoint for monitoring
// - Logging integration with Kratos log interface
// - TLS support for secure connections
package sse

import (
	"context"
	"net/http"

	kratosTransport "github.com/go-kratos/kratos/v2/transport"
)

const (
	KindSSE = "sse"
)

var _ Transporter = &Transport{}

// Transporter is a SSE transporter.
type Transporter interface {
	kratosTransport.Transporter
	Request() *http.Request
}

// headerCarrier is a header carrier.
type headerCarrier http.Header

// Get returns the value associated with the passed key.
func (hc headerCarrier) Get(key string) string {
	return http.Header(hc).Get(key)
}

// Set stores the key-value pair.
func (hc headerCarrier) Set(key, value string) {
	http.Header(hc).Set(key, value)
}

// Add adds the key-value pair.
func (hc headerCarrier) Add(key, value string) {
	http.Header(hc).Add(key, value)
}

// Values returns the values associated with the passed key.
func (hc headerCarrier) Values(key string) []string {
	return http.Header(hc).Values(key)
}

// Keys returns the keys stored in this carrier.
func (hc headerCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for k := range hc {
		keys = append(keys, k)
	}
	return keys
}

// Transport is a SSE transport.
type Transport struct {
	req       *http.Request
	reqHeader kratosTransport.Header
	endpoint  string
	operation string
}

// Kind returns the transport kind.
func (tr *Transport) Kind() kratosTransport.Kind {
	return KindSSE
}

// Endpoint returns the transport endpoint.
func (tr *Transport) Endpoint() string {
	return tr.endpoint
}

// Operation returns the transport operation.
func (tr *Transport) Operation() string {
	return tr.operation
}

// Request returns the HTTP request.
func (tr *Transport) Request() *http.Request {
	return tr.req
}

// RequestHeader returns the request header.
func (tr *Transport) RequestHeader() kratosTransport.Header {
	return tr.reqHeader
}

// ReplyHeader returns the reply header.
func (tr *Transport) ReplyHeader() kratosTransport.Header {
	return nil
}

// NewTransport creates a new SSE transport.
func NewTransport(ctx context.Context, req *http.Request, endpoint, operation string) *Transport {
	return &Transport{
		req:       req,
		reqHeader: headerCarrier(req.Header),
		endpoint:  endpoint,
		operation: operation,
	}
}
