package sse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Event represents a server-sent event.
type Event struct {
	// ID is the event ID.
	ID []byte
	// Event is the event type.
	Event []byte
	// Data is the event data.
	Data []byte
	// Retry is the reconnection time in milliseconds.
	Retry []byte
	// Timestamp is the event timestamp.
	Timestamp time.Time
}

// NewEvent creates a new event.
func NewEvent() *Event {
	return &Event{
		Timestamp: time.Now(),
	}
}

// NewEventWithData creates a new event with data.
func NewEventWithData(data []byte) *Event {
	return &Event{
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewEventWithEvent creates a new event with event type.
func NewEventWithEvent(event []byte, data []byte) *Event {
	return &Event{
		Event:     event,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewEventWithID creates a new event with ID.
func NewEventWithID(id []byte, data []byte) *Event {
	return &Event{
		ID:        id,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewEventWithAll creates a new event with all fields.
func NewEventWithAll(id []byte, event []byte, data []byte, retry []byte) *Event {
	return &Event{
		ID:        id,
		Event:     event,
		Data:      data,
		Retry:     retry,
		Timestamp: time.Now(),
	}
}

// MarshalJSON marshals the event to JSON.
func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        string(e.ID),
		"event":     string(e.Event),
		"data":      string(e.Data),
		"retry":     string(e.Retry),
		"timestamp": e.Timestamp,
	})
}

// UnmarshalJSON unmarshals the event from JSON.
func (e *Event) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	if id, ok := m["id"].(string); ok {
		e.ID = []byte(id)
	}

	if event, ok := m["event"].(string); ok {
		e.Event = []byte(event)
	}

	if data, ok := m["data"].(string); ok {
		e.Data = []byte(data)
	}

	if retry, ok := m["retry"].(string); ok {
		e.Retry = []byte(retry)
	}

	if timestamp, ok := m["timestamp"].(string); ok {
		t, err := time.Parse(time.RFC3339, timestamp)
		if err == nil {
			e.Timestamp = t
		}
	}

	return nil
}

// String returns the string representation of the event.
func (e *Event) String() string {
	var buf bytes.Buffer

	if len(e.ID) > 0 {
		fmt.Fprintf(&buf, "id: %s\n", e.ID)
	}

	if len(e.Event) > 0 {
		fmt.Fprintf(&buf, "event: %s\n", e.Event)
	}

	if len(e.Data) > 0 {
		// Split data into lines
		lines := bytes.Split(e.Data, []byte("\n"))
		for _, line := range lines {
			fmt.Fprintf(&buf, "data: %s\n", line)
		}
	}

	if len(e.Retry) > 0 {
		fmt.Fprintf(&buf, "retry: %s\n", e.Retry)
	}

	buf.WriteString("\n")

	return buf.String()
}

// Bytes returns the byte representation of the event.
func (e *Event) Bytes() []byte {
	return []byte(e.String())
}
