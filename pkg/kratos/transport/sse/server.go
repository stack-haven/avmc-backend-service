package sse

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/gorilla/mux"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
	_ http.Handler         = (*Server)(nil)
)

// MessagePayload is the message payload type.
type MessagePayload any

// Server is a SSE server.
type Server struct {
	*http.Server

	lis      net.Listener
	tlsConf  *tls.Config
	endpoint *url.URL

	network     string
	address     string
	path        string
	streamIdKey string

	timeout time.Duration

	err    error
	codec  encoding.Codec
	logger log.Logger

	router      *mux.Router
	strictSlash bool

	headers    map[string]string
	eventTTL   time.Duration
	bufferSize int

	encodeBase64 bool
	splitData    bool
	autoStream   bool
	autoReplay   bool

	subscribeFunc   SubscribeFunc
	unsubscribeFunc SubscribeFunc

	middlewares []func(http.Handler) http.Handler

	streamMgr *StreamManager

	// Metrics
	activeConnections atomic.Int64
	eventsSent        atomic.Int64
	errorCount        atomic.Int64
	connectionsOpened atomic.Int64
	connectionsClosed atomic.Int64
}

// NewServer creates a new SSE server.
func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		network:     "tcp",
		address:     ":0",
		timeout:     1 * time.Second,
		router:      mux.NewRouter(),
		strictSlash: true,
		path:        "/stream",
		streamIdKey: "stream",

		bufferSize:   DefaultBufferSize,
		encodeBase64: false,

		autoStream: false,
		autoReplay: true,
		headers:    map[string]string{},

		logger:      log.NewStdLogger(os.Stdout),
		middlewares: make([]func(http.Handler) http.Handler, 0),
		streamMgr:   NewStreamManager(),
	}

	srv.init(opts...)

	return srv
}

// Name returns the server name.
func (s *Server) Name() string {
	return KindSSE
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) error {
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}

	if s.err != nil {
		return s.err
	}

	s.BaseContext = func(net.Listener) context.Context {
		return ctx
	}

	// Log server start
	s.logger.Log(log.LevelInfo, "msg", "SSE server listening on", "addr", s.lis.Addr().String())

	s.HandleServeHTTP(s.path)

	var err error
	if s.tlsConf != nil {
		err = s.ServeTLS(s.lis, "", "")
	} else {
		err = s.Serve(s.lis)
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Stop stops the server gracefully.
func (s *Server) Stop(ctx context.Context) error {
	// Log server stop
	s.logger.Log(log.LevelInfo, "msg", "SSE server stopping...")

	// Create a timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Clean up streams with timeout
	streamCleanupDone := make(chan struct{})
	go func() {
		defer close(streamCleanupDone)
		// Clean up streams
		s.streamMgr.Range(func(stream *Stream) {
			stream.Close()
		})
		s.streamMgr.Clean()
	}()

	// Wait for stream cleanup or timeout
	select {
	case <-streamCleanupDone:
		s.logger.Log(log.LevelInfo, "msg", "SSE stream cleanup completed")
	case <-shutdownCtx.Done():
		s.logger.Log(log.LevelWarn, "msg", "SSE stream cleanup timed out", "err", shutdownCtx.Err())
	}

	// Shutdown HTTP server with timeout
	err := s.Shutdown(shutdownCtx)
	if err != nil {
		s.logger.Log(log.LevelError, "msg", "SSE server shutdown failed", "err", err)
		return err
	}

	// Close listener to prevent new connections
	if s.lis != nil {
		if closeErr := s.lis.Close(); closeErr != nil {
			s.logger.Log(log.LevelError, "msg", "SSE listener close failed", "err", closeErr)
		}
	}

	// Reset error state
	s.err = nil

	// Log server stopped
	s.logger.Log(log.LevelInfo, "msg", "SSE server stopped gracefully")

	return nil
}

// Endpoint returns the server endpoint.
func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}
	return s.endpoint, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// processEvent processes an event before sending it.
func (s *Server) processEvent(event *Event) *Event {
	if s.encodeBase64 {
		// Encode data as base64
		encoded := base64.StdEncoding.EncodeToString(event.Data)
		event.Data = []byte(encoded)
	}
	return event
}

// Publish publishes an event to a stream.
func (s *Server) Publish(_ context.Context, streamId StreamID, event *Event) {
	stream := s.streamMgr.Get(streamId)
	if stream == nil {
		s.errorCount.Add(1)
		return
	}

	stream.Send(s.processEvent(event))
	s.eventsSent.Add(1)
}

// TryPublish tries to publish an event to a stream.
func (s *Server) TryPublish(_ context.Context, streamId StreamID, event *Event) bool {
	stream := s.streamMgr.Get(streamId)
	if stream == nil {
		s.errorCount.Add(1)
		return false
	}

	if stream.TrySend(s.processEvent(event)) {
		s.eventsSent.Add(1)
		return true
	}
	s.errorCount.Add(1)
	return false
}

// PublishData publishes data to a stream.
func (s *Server) PublishData(ctx context.Context, streamId StreamID, data MessagePayload) error {
	event := NewEvent()

	if data != nil {
		var err error
		event.Data, err = encodeData(s.codec, data)
		if err != nil {
			s.logger.Log(log.LevelError, "msg", "SSE server failed to encode data", "stream_id", streamId, "err", err)
			return err
		}
	}

	stream := s.streamMgr.Get(streamId)
	if stream == nil {
		s.logger.Log(log.LevelWarn, "msg", "SSE server tried to publish to non-existent stream", "stream_id", streamId)
		return errors.New(404, "STREAM_NOT_FOUND", "stream not found")
	}

	s.Publish(ctx, streamId, event)
	s.logger.Log(log.LevelDebug, "msg", "SSE server published data to stream", "stream_id", streamId)

	return nil
}

// Notify notifies all streams with an event.
func (s *Server) Notify(_ context.Context, event *Event) {
	s.streamMgr.Range(func(stream *Stream) {
		stream.Send(s.processEvent(event))
	})
}

// NotifyData notifies all streams with data.
func (s *Server) NotifyData(_ context.Context, data MessagePayload) error {
	event := NewEvent()

	if data != nil {
		var err error
		event.Data, err = encodeData(s.codec, data)
		if err != nil {
			s.logger.Log(log.LevelError, "msg", "SSE server failed to encode data for notification", "err", err)
			return err
		}
	}

	processedEvent := s.processEvent(event)
	streamCount := 0

	s.streamMgr.Range(func(stream *Stream) {
		stream.Send(processedEvent)
		streamCount++
	})

	s.logger.Log(log.LevelDebug, "msg", "SSE server notified all streams", "stream_count", streamCount)

	return nil
}

// CreateStream creates a new stream.
func (s *Server) CreateStream(streamId StreamID) *Stream {
	stream := s.streamMgr.Get(streamId)
	if stream != nil {
		return stream
	}

	stream = s.createStream(streamId)

	s.streamMgr.Add(stream)

	return stream
}

// HandleServeHTTP handles the SSE HTTP endpoint.
func (s *Server) HandleServeHTTP(path string) {
	s.router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		handleSSE(s, w, r)
	})
}

// RegisterSSEPath registers a new SSE path for a specific business interface.
func (s *Server) RegisterSSEPath(path string, businessType string) {
	s.router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		// Add business type to request context for later use
		r = r.WithContext(context.WithValue(r.Context(), "business_type", businessType))
		handleSSE(s, w, r)
	})
}

// adjustAddress adjusts the address.
func adjustAddress(address string, lis net.Listener) (string, error) {
	if address == ":0" {
		return lis.Addr().String(), nil
	}
	return address, nil
}

// listenAndEndpoint creates a listener and endpoint URL.
func (s *Server) listenAndEndpoint() error {
	if s.lis == nil {
		lis, err := net.Listen(s.network, s.address)
		if err != nil {
			s.logger.Log(log.LevelError, "msg", "SSE server failed to create listener", "network", s.network, "address", s.address, "err", err)
			s.err = err
			return err
		}
		s.lis = lis
		s.logger.Log(log.LevelInfo, "msg", "SSE server created listener", "network", s.network, "address", s.address, "actual_addr", lis.Addr().String())
	}

	if s.endpoint == nil {
		// Adjust address
		addr, err := adjustAddress(s.address, s.lis)
		if err != nil {
			s.logger.Log(log.LevelError, "msg", "SSE server failed to adjust address", "address", s.address, "err", err)
			s.err = err
			return err
		}

		// Create endpoint URL
		scheme := "http"
		if s.tlsConf != nil {
			scheme = "https"
			s.logger.Log(log.LevelInfo, "msg", "SSE server using HTTPS")
		}
		endpointURL := scheme + "://" + addr
		s.endpoint, err = url.Parse(endpointURL)
		if err != nil {
			s.logger.Log(log.LevelError, "msg", "SSE server failed to parse endpoint URL", "url", endpointURL, "err", err)
			s.err = err
			return err
		}
		s.logger.Log(log.LevelInfo, "msg", "SSE server created endpoint", "endpoint", s.endpoint.String())
	}

	return nil
}

// healthCheck handles health check requests.
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// metrics handles metrics requests.
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"active_connections": s.activeConnections.Load(),
		"events_sent":        s.eventsSent.Load(),
		"error_count":        s.errorCount.Load(),
		"connections_opened": s.connectionsOpened.Load(),
		"connections_closed": s.connectionsClosed.Load(),
		"total_streams":      s.streamMgr.Count(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// init initializes the server.
func (s *Server) init(opts ...ServerOption) {
	for _, o := range opts {
		o(s)
	}

	s.router.StrictSlash(s.strictSlash)
	s.router.NotFoundHandler = http.DefaultServeMux
	s.router.MethodNotAllowedHandler = http.DefaultServeMux

	// Register health check endpoint
	s.router.HandleFunc("/healthz", s.healthCheck)

	// Register metrics endpoint
	s.router.HandleFunc("/metrics", s.metrics)

	// Apply middlewares
	handler := http.Handler(s.router)
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		handler = s.middlewares[i](handler)
	}

	s.Server = &http.Server{
		Handler:   handler,
		TLSConfig: s.tlsConf,
	}
}

// listen listens on the network address.
func (s *Server) listen() error {
	if s.lis == nil {
		lis, err := net.Listen(s.network, s.address)
		if err != nil {
			return err
		}
		s.lis = lis
	}

	return nil
}

// createStream creates a new stream.
func (s *Server) createStream(streamId StreamID) *Stream {
	stream := NewStream(streamId, s.bufferSize, s.autoReplay, s.autoStream, s.subscribeFunc, s.unsubscribeFunc)
	return stream
}

// encodeData encodes data using the codec.
func encodeData(codec encoding.Codec, data any) ([]byte, error) {
	if codec == nil {
		// Use default JSON codec
		return json.Marshal(data)
	}
	return codec.Marshal(data)
}
