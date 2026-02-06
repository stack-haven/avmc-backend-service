package sse

// SubscribeFunc is a subscribe function type.
type SubscribeFunc func(streamID StreamID)

// SubscriberFunction is an alias for SubscribeFunc.
type SubscriberFunction SubscribeFunc
