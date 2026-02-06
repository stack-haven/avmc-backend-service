package sse

import (
	"context"
	"testing"
	"time"
)

func TestServer_Creation(t *testing.T) {
	srv := NewServer(
		WithAddress(":0"),
		WithPath("/sse"),
		WithBufferSize(1024),
		WithAutoReplay(true),
	)

	if srv == nil {
		t.Fatal("NewServer returned nil")
	}

	if srv.Name() != KindSSE {
		t.Errorf("Server.Name() = %v, want %v", srv.Name(), KindSSE)
	}
}

func TestServer_Endpoint(t *testing.T) {
	srv := NewServer(
		WithAddress(":0"),
	)

	endpoint, err := srv.Endpoint()
	if err != nil {
		t.Fatalf("Server.Endpoint() error: %v", err)
	}

	if endpoint == nil {
		t.Fatal("Server.Endpoint() returned nil")
	}

	if endpoint.Scheme != "http" {
		t.Errorf("Endpoint scheme = %v, want %v", endpoint.Scheme, "http")
	}
}

func TestServer_CreateStream(t *testing.T) {
	srv := NewServer()

	streamID := StreamID("test-stream")
	stream := srv.CreateStream(streamID)

	if stream == nil {
		t.Fatal("Server.CreateStream returned nil")
	}

	if stream.ID != streamID {
		t.Errorf("Stream ID = %v, want %v", stream.ID, streamID)
	}

	// Create stream again (should return the same stream)
	stream2 := srv.CreateStream(streamID)
	if stream2 != stream {
		t.Error("Server.CreateStream should return the same stream for existing ID")
	}
}

func TestServer_Publish(t *testing.T) {
	srv := NewServer()

	streamID := StreamID("test-stream")
	stream := srv.CreateStream(streamID)
	stream.Run()
	defer stream.Close()

	event := NewEventWithData([]byte("test data"))
	srv.Publish(context.Background(), streamID, event)

	// Check if event was published
	select {
	case receivedEvent := <-stream.Events():
		if string(receivedEvent.Data) != string(event.Data) {
			t.Errorf("Received event data = %v, want %v", string(receivedEvent.Data), string(event.Data))
		}
	case <-time.After(1 * time.Second):
		t.Error("No event received from stream after publish")
	}
}

func TestServer_Notify(t *testing.T) {
	srv := NewServer()

	// Create multiple streams
	streamIDs := []StreamID{"stream1", "stream2"}
	streams := make([]*Stream, len(streamIDs))

	for i, id := range streamIDs {
		stream := srv.CreateStream(id)
		stream.Run()
		defer stream.Close()
		streams[i] = stream
	}

	event := NewEventWithData([]byte("test notification"))
	srv.Notify(context.Background(), event)

	// Check if all streams received the notification
	for i, stream := range streams {
		select {
		case receivedEvent := <-stream.Events():
			if string(receivedEvent.Data) != string(event.Data) {
				t.Errorf("Stream %d received event data = %v, want %v", i, string(receivedEvent.Data), string(event.Data))
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Stream %d did not receive notification", i)
		}
	}
}

func TestServer_PublishData(t *testing.T) {
	srv := NewServer()

	streamID := StreamID("test-stream")
	stream := srv.CreateStream(streamID)
	stream.Run()
	defer stream.Close()

	data := map[string]string{"message": "test data"}
	err := srv.PublishData(context.Background(), streamID, data)
	if err != nil {
		t.Fatalf("Server.PublishData error: %v", err)
	}

	// Check if data was published
	select {
	case receivedEvent := <-stream.Events():
		if len(receivedEvent.Data) == 0 {
			t.Error("Received event data is empty")
		}
	case <-time.After(1 * time.Second):
		t.Error("No event received from stream after PublishData")
	}
}

func TestServer_NotifyData(t *testing.T) {
	srv := NewServer()

	// Create stream
	streamID := StreamID("test-stream")
	stream := srv.CreateStream(streamID)
	stream.Run()
	defer stream.Close()

	data := map[string]string{"message": "test notification"}
	err := srv.NotifyData(context.Background(), data)
	if err != nil {
		t.Fatalf("Server.NotifyData error: %v", err)
	}

	// Check if data was notified
	select {
	case receivedEvent := <-stream.Events():
		if len(receivedEvent.Data) == 0 {
			t.Error("Received event data is empty")
		}
	case <-time.After(1 * time.Second):
		t.Error("No event received from stream after NotifyData")
	}
}
