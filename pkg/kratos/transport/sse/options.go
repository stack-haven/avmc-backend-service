package sse

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// ServerOption is a server option.
type ServerOption func(*Server)

// WithNetwork sets the network address of the server.
func WithNetwork(network string) ServerOption {
	return func(s *Server) {
		s.network = network
	}
}

// WithAddress sets the address of the server.
func WithAddress(address string) ServerOption {
	return func(s *Server) {
		s.address = address
	}
}

// WithTLSConfig sets the TLS config of the server.
func WithTLSConfig(tlsConf *tls.Config) ServerOption {
	return func(s *Server) {
		s.tlsConf = tlsConf
	}
}

// WithTimeout sets the timeout of the server.
func WithTimeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.timeout = timeout
	}
}

// WithListener sets the listener of the server.
func WithListener(lis net.Listener) ServerOption {
	return func(s *Server) {
		s.lis = lis
	}
}

// WithPath sets the path of the server.
func WithPath(path string) ServerOption {
	return func(s *Server) {
		s.path = path
	}
}

// WithBufferSize sets the buffer size of the server.
func WithBufferSize(bufferSize int) ServerOption {
	return func(s *Server) {
		s.bufferSize = bufferSize
	}
}

// WithAutoReplay sets the auto replay flag of the server.
func WithAutoReplay(autoReplay bool) ServerOption {
	return func(s *Server) {
		s.autoReplay = autoReplay
	}
}

// WithAutoStream sets the auto stream flag of the server.
func WithAutoStream(autoStream bool) ServerOption {
	return func(s *Server) {
		s.autoStream = autoStream
	}
}

// WithEncodeBase64 sets the encode base64 flag of the server.
func WithEncodeBase64(encodeBase64 bool) ServerOption {
	return func(s *Server) {
		s.encodeBase64 = encodeBase64
	}
}

// WithSplitData sets the split data flag of the server.
func WithSplitData(splitData bool) ServerOption {
	return func(s *Server) {
		s.splitData = splitData
	}
}

// WithHeaders sets the headers of the server.
func WithHeaders(headers map[string]string) ServerOption {
	return func(s *Server) {
		s.headers = headers
	}
}

// WithEventTTL sets the event TTL of the server.
func WithEventTTL(eventTTL time.Duration) ServerOption {
	return func(s *Server) {
		s.eventTTL = eventTTL
	}
}

// WithStreamIDKey sets the stream ID key of the server.
func WithStreamIDKey(streamIDKey string) ServerOption {
	return func(s *Server) {
		s.streamIdKey = streamIDKey
	}
}

// WithSubscribeFunc sets the subscribe function of the server.
func WithSubscribeFunc(subscribeFunc SubscribeFunc) ServerOption {
	return func(s *Server) {
		s.subscribeFunc = subscribeFunc
	}
}

// WithUnsubscribeFunc sets the unsubscribe function of the server.
func WithUnsubscribeFunc(unsubscribeFunc SubscribeFunc) ServerOption {
	return func(s *Server) {
		s.unsubscribeFunc = unsubscribeFunc
	}
}

// WithLogger sets the logger of the server.
func WithLogger(logger log.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// WithMiddleware adds a middleware to the server.
func WithMiddleware(middleware func(http.Handler) http.Handler) ServerOption {
	return func(s *Server) {
		s.middlewares = append(s.middlewares, middleware)
	}
}

// WithMiddlewares adds multiple middlewares to the server.
func WithMiddlewares(middlewares ...func(http.Handler) http.Handler) ServerOption {
	return func(s *Server) {
		s.middlewares = append(s.middlewares, middlewares...)
	}
}
