package sse

import (
	"testing"
)

func TestStream_Creation(t *testing.T) {
	streamID := StreamID("test-stream")
	bufferSize := 10
	autoReplay := true
	autoStream := false

	var subscribeCalled, unsubscribeCalled bool
	subscribeFunc := func(id StreamID) {
		subscribeCalled = true
		if id != streamID {
			t.Errorf("subscribeFunc called with wrong ID: %v, want %v", id, streamID)
		}
	}

	unsubscribeFunc := func(id StreamID) {
		unsubscribeCalled = true
		if id != streamID {
			t.Errorf("unsubscribeFunc called with wrong ID: %v, want %v", id, streamID)
		}
	}

	stream := NewStream(streamID, bufferSize, autoReplay, autoStream, subscribeFunc, unsubscribeFunc)

	if stream.ID != streamID {
		t.Errorf("Stream ID = %v, want %v", stream.ID, streamID)
	}

	stream.Run()
	if !subscribeCalled {
		t.Error("subscribeFunc was not called")
	}

	stream.Close()
	if !unsubscribeCalled {
		t.Error("unsubscribeFunc was not called")
	}
}

func TestStream_Send(t *testing.T) {
	streamID := StreamID("test-stream")
	stream := NewStream(streamID, 10, true, false, nil, nil)
	stream.Run()
	defer stream.Close()

	event := NewEventWithData([]byte("test data"))
	stream.Send(event)

	// Check if event was sent to channel
	select {
	case receivedEvent := <-stream.Events():
		if string(receivedEvent.Data) != string(event.Data) {
			t.Errorf("Received event data = %v, want %v", string(receivedEvent.Data), string(event.Data))
		}
	default:
		t.Error("No event received from stream")
	}
}

func TestStream_TrySend(t *testing.T) {
	streamID := StreamID("test-stream")
	stream := NewStream(streamID, 1, true, false, nil, nil)
	stream.Run()
	defer stream.Close()

	// Send first event
	event1 := NewEventWithData([]byte("test data 1"))
	if !stream.TrySend(event1) {
		t.Error("TrySend should have succeeded for first event")
	}

	// Send second event (should fail because buffer size is 1)
	event2 := NewEventWithData([]byte("test data 2"))
	if stream.TrySend(event2) {
		t.Error("TrySend should have failed for second event due to buffer size")
	}
}

func TestStreamManager(t *testing.T) {
	streamMgr := NewStreamManager()

	// Create and add stream
	streamID := StreamID("test-stream")
	stream := NewStream(streamID, 10, true, false, nil, nil)
	streamMgr.Add(stream)

	// Get stream
	retrievedStream := streamMgr.Get(streamID)
	if retrievedStream == nil {
		t.Error("StreamManager.Get should have returned the stream")
	}
	if retrievedStream.ID != streamID {
		t.Errorf("Retrieved stream ID = %v, want %v", retrievedStream.ID, streamID)
	}

	// Count streams
	if streamMgr.Count() != 1 {
		t.Errorf("StreamManager.Count = %v, want %v", streamMgr.Count(), 1)
	}

	// Delete stream
	streamMgr.Delete(streamID)
	if streamMgr.Get(streamID) != nil {
		t.Error("StreamManager.Get should have returned nil after delete")
	}
	if streamMgr.Count() != 0 {
		t.Errorf("StreamManager.Count = %v, want %v", streamMgr.Count(), 0)
	}
}

func TestStreamManager_Range(t *testing.T) {
	streamMgr := NewStreamManager()

	// Add multiple streams
	streamIDs := []StreamID{"stream1", "stream2", "stream3"}
	for _, id := range streamIDs {
		stream := NewStream(id, 10, true, false, nil, nil)
		streamMgr.Add(stream)
	}

	// Count streams using Range
	count := 0
	streamMgr.Range(func(stream *Stream) {
		count++
	})

	if count != len(streamIDs) {
		t.Errorf("Range count = %v, want %v", count, len(streamIDs))
	}
}
