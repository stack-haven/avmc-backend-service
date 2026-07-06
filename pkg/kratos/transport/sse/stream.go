package sse

import (
	"sync"
)

const (
	// DefaultBufferSize is the default buffer size.
	DefaultBufferSize = 1024
)

// StreamID is the stream ID type.
type StreamID string

// Stream is a SSE stream.
type Stream struct {
	// ID is the stream ID.
	ID StreamID
	// event is the event channel.
	event chan *Event
	// quit is the quit channel.
	quit chan struct{}
	// bufferSize is the buffer size.
	bufferSize int
	// autoReplay is the auto replay flag.
	autoReplay bool
	// autoStream is the auto stream flag.
	autoStream bool
	// subscribeFunc is the subscribe function.
	subscribeFunc SubscribeFunc
	// unsubscribeFunc is the unsubscribe function.
	unsubscribeFunc SubscribeFunc
	// events is the event history.
	events []*Event
	// mutex is the mutex for events.
	mutex sync.RWMutex
	// closeOnce makes stream shutdown safe across handler and server cleanup.
	closeOnce sync.Once
}

// NewStream creates a new stream.
func NewStream(id StreamID, bufferSize int, autoReplay, autoStream bool, subscribeFunc, unsubscribeFunc SubscribeFunc) *Stream {
	return &Stream{
		ID:              id,
		event:           make(chan *Event, bufferSize),
		quit:            make(chan struct{}),
		bufferSize:      bufferSize,
		autoReplay:      autoReplay,
		autoStream:      autoStream,
		subscribeFunc:   subscribeFunc,
		unsubscribeFunc: unsubscribeFunc,
		events:          make([]*Event, 0, bufferSize),
	}
}

// Run starts the stream.
func (s *Stream) Run() {
	if s.subscribeFunc != nil {
		s.subscribeFunc(s.ID)
	}
}

// Close closes the stream.
func (s *Stream) Close() {
	s.closeOnce.Do(func() {
		close(s.quit)
		if s.unsubscribeFunc != nil {
			s.unsubscribeFunc(s.ID)
		}
	})
}

// Send sends an event to the stream.
func (s *Stream) Send(event *Event) {
	select {
	case <-s.quit:
		return
	case s.event <- event:
		// Store event for replay if autoReplay is true
		if s.autoReplay {
			s.mutex.Lock()
			s.events = append(s.events, event)
			// Limit event history to buffer size
			if len(s.events) > s.bufferSize {
				s.events = s.events[len(s.events)-s.bufferSize:]
			}
			s.mutex.Unlock()
		}
	}
}

// TrySend tries to send an event to the stream.
func (s *Stream) TrySend(event *Event) bool {
	select {
	case <-s.quit:
		return false
	case s.event <- event:
		// Store event for replay if autoReplay is true
		if s.autoReplay {
			s.mutex.Lock()
			s.events = append(s.events, event)
			// Limit event history to buffer size
			if len(s.events) > s.bufferSize {
				s.events = s.events[len(s.events)-s.bufferSize:]
			}
			s.mutex.Unlock()
		}
		return true
	default:
		return false
	}
}

// Events returns the event channel.
func (s *Stream) Events() chan *Event {
	return s.event
}

// Quit returns the quit channel.
func (s *Stream) Quit() chan struct{} {
	return s.quit
}

// Events returns the event history.
func (s *Stream) EventsHistory() []*Event {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	events := make([]*Event, len(s.events))
	copy(events, s.events)
	return events
}

// StreamManager manages streams.
type StreamManager struct {
	// streams is the stream map.
	streams map[StreamID]*Stream
	// mutex is the mutex for streams.
	mutex sync.RWMutex
}

// NewStreamManager creates a new stream manager.
func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[StreamID]*Stream),
	}
}

// Add adds a stream to the manager.
func (sm *StreamManager) Add(stream *Stream) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.streams[stream.ID] = stream
}

// Get gets a stream from the manager.
func (sm *StreamManager) Get(id StreamID) *Stream {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.streams[id]
}

// Delete deletes a stream from the manager.
func (sm *StreamManager) Delete(id StreamID) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, ok := sm.streams[id]; ok {
		stream.Close()
		delete(sm.streams, id)
	}
}

// Range ranges over the streams.
func (sm *StreamManager) Range(f func(stream *Stream)) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for _, stream := range sm.streams {
		f(stream)
	}
}

// Count returns the number of streams.
func (sm *StreamManager) Count() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return len(sm.streams)
}

// Clean cleans up closed streams.
func (sm *StreamManager) Clean() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	for id, stream := range sm.streams {
		select {
		case <-stream.Quit():
			delete(sm.streams, id)
		default:
			// Stream is still active
		}
	}
}
