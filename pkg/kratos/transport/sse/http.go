package sse

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"time"
)

// setSSEHeaders sets the SSE response headers.
func setSSEHeaders(w http.ResponseWriter, headers map[string]string) {
	// Set default SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Set custom headers
	for key, value := range headers {
		w.Header().Set(key, value)
	}
}

// sendEvent sends an event to the client.
func sendEvent(w http.ResponseWriter, event *Event) (int, error) {
	// Write event to response
	return w.Write(event.Bytes())
}

// sendComment sends a comment to the client to keep the connection alive.
func sendComment(w http.ResponseWriter) (int, error) {
	return w.Write([]byte(": keep-alive\n\n"))
}

// handleSSE handles an SSE request.
func handleSSE(s *Server, w http.ResponseWriter, r *http.Request) {
	// Increment connection counters
	s.connectionsOpened.Add(1)
	s.activeConnections.Add(1)
	defer func() {
		s.activeConnections.Add(-1)
		s.connectionsClosed.Add(1)
	}()

	// Set SSE headers
	setSSEHeaders(w, s.headers)

	// Get stream ID from request
	streamID := StreamID(r.URL.Query().Get(s.streamIdKey))
	if streamID == "" {
		// Generate a random stream ID if not provided
		streamID = StreamID(strconv.FormatInt(time.Now().UnixNano(), 10))
	}

	// Create or get stream
	stream := s.streamMgr.Get(streamID)
	if stream == nil {
		stream = s.createStream(streamID)
		s.streamMgr.Add(stream)
		stream.Run()
	}

	// Flush response writer to send headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Response writer does not support flushing
		s.errorCount.Add(1)
		return
	}
	flusher.Flush()

	// Send event history if autoReplay is true
	if s.autoReplay {
		events := stream.EventsHistory()
		for _, event := range events {
			if _, err := sendEvent(w, event); err != nil {
				// Client disconnected
				stream.Close()
				s.streamMgr.Delete(streamID)
				s.errorCount.Add(1)
				return
			}
			flusher.Flush()
			s.eventsSent.Add(1)
		}
	}

	// Start sending events
	keepAliveTicker := time.NewTicker(30 * time.Second)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			stream.Close()
			s.streamMgr.Delete(streamID)
			return
		case <-stream.Quit():
			// Stream closed
			return
		case event, ok := <-stream.Events():
			if !ok {
				// Event channel closed
				return
			}

			// Process event
			processedEvent := s.process(event)

			// Send event
			if _, err := sendEvent(w, processedEvent); err != nil {
				// Client disconnected
				stream.Close()
				s.streamMgr.Delete(streamID)
				s.errorCount.Add(1)
				return
			}
			flusher.Flush()
			s.eventsSent.Add(1)
		case <-keepAliveTicker.C:
			// Send keep-alive comment
			if _, err := sendComment(w); err != nil {
				// Client disconnected
				stream.Close()
				s.streamMgr.Delete(streamID)
				s.errorCount.Add(1)
				return
			}
			flusher.Flush()
		}
	}
}

// process processes an event before sending it.
func (s *Server) process(event *Event) *Event {
	if s.encodeBase64 {
		// Encode data as base64
		encoded := base64.StdEncoding.EncodeToString(event.Data)
		event.Data = []byte(encoded)
	}
	return event
}
