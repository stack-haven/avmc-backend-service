package sse

import (
	"encoding/json"
	"testing"
)

func TestEvent_Creation(t *testing.T) {
	tests := []struct {
		name string
		fn   func() *Event
		want *Event
	}{
		{
			name: "NewEvent",
			fn:   NewEvent,
			want: &Event{Timestamp: NewEvent().Timestamp},
		},
		{
			name: "NewEventWithData",
			fn:   func() *Event { return NewEventWithData([]byte("test data")) },
			want: &Event{Data: []byte("test data"), Timestamp: NewEventWithData([]byte("test data")).Timestamp},
		},
		{
			name: "NewEventWithEvent",
			fn:   func() *Event { return NewEventWithEvent([]byte("test event"), []byte("test data")) },
			want: &Event{Event: []byte("test event"), Data: []byte("test data"), Timestamp: NewEventWithEvent([]byte("test event"), []byte("test data")).Timestamp},
		},
		{
			name: "NewEventWithID",
			fn:   func() *Event { return NewEventWithID([]byte("test id"), []byte("test data")) },
			want: &Event{ID: []byte("test id"), Data: []byte("test data"), Timestamp: NewEventWithID([]byte("test id"), []byte("test data")).Timestamp},
		},
		{
			name: "NewEventWithAll",
			fn: func() *Event {
				return NewEventWithAll([]byte("test id"), []byte("test event"), []byte("test data"), []byte("1000"))
			},
			want: &Event{ID: []byte("test id"), Event: []byte("test event"), Data: []byte("test data"), Retry: []byte("1000"), Timestamp: NewEventWithAll([]byte("test id"), []byte("test event"), []byte("test data"), []byte("1000")).Timestamp},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := tt.fn()
			if tt.want.ID != nil && string(ev.ID) != string(tt.want.ID) {
				t.Errorf("ID = %v, want %v", string(ev.ID), string(tt.want.ID))
			}
			if tt.want.Event != nil && string(ev.Event) != string(tt.want.Event) {
				t.Errorf("Event = %v, want %v", string(ev.Event), string(tt.want.Event))
			}
			if tt.want.Data != nil && string(ev.Data) != string(tt.want.Data) {
				t.Errorf("Data = %v, want %v", string(ev.Data), string(tt.want.Data))
			}
			if tt.want.Retry != nil && string(ev.Retry) != string(tt.want.Retry) {
				t.Errorf("Retry = %v, want %v", string(ev.Retry), string(tt.want.Retry))
			}
		})
	}
}

func TestEvent_MarshalJSON(t *testing.T) {
	event := NewEventWithAll([]byte("test id"), []byte("test event"), []byte("test data"), []byte("1000"))

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	var unmarshaled Event
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	if string(unmarshaled.ID) != string(event.ID) {
		t.Errorf("ID = %v, want %v", string(unmarshaled.ID), string(event.ID))
	}
	if string(unmarshaled.Event) != string(event.Event) {
		t.Errorf("Event = %v, want %v", string(unmarshaled.Event), string(event.Event))
	}
	if string(unmarshaled.Data) != string(event.Data) {
		t.Errorf("Data = %v, want %v", string(unmarshaled.Data), string(event.Data))
	}
	if string(unmarshaled.Retry) != string(event.Retry) {
		t.Errorf("Retry = %v, want %v", string(unmarshaled.Retry), string(event.Retry))
	}
}

func TestEvent_String(t *testing.T) {
	event := NewEventWithAll([]byte("test id"), []byte("test event"), []byte("test data"), []byte("1000"))

	str := event.String()
	expected := "id: test id\nevent: test event\ndata: test data\nretry: 1000\n\n"

	if str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}

func TestEvent_Bytes(t *testing.T) {
	event := NewEventWithData([]byte("test data"))

	bytes := event.Bytes()
	expected := []byte("data: test data\n\n")

	if string(bytes) != string(expected) {
		t.Errorf("Bytes() = %q, want %q", bytes, expected)
	}
}
