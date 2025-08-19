package viewer

import "context"

// Viewer defines the interface for a context viewer.
type Viewer interface {
	// DomainID returns the domain ID of the viewer.
	DomainID() (int64, bool)
}

// domainViewer implements the Viewer interface.
type domainViewer struct {
	domainID int64
}

// NewViewer returns a new viewer with the given domain ID.
func NewViewer(domainID int64) Viewer {
	return &domainViewer{domainID: domainID}
}

// DomainID returns the domain ID of the viewer.
func (v *domainViewer) DomainID() (int64, bool) {
	if v.domainID > 0 {
		return v.domainID, true
	}
	return 0, false
}

// key is the context key for the viewer.
type key string

const viewerKey key = "viewer"

// NewContext returns a new context with the given viewer.
func NewContext(ctx context.Context, v Viewer) context.Context {
	return context.WithValue(ctx, viewerKey, v)
}

// FromContext returns the viewer from the given context.
func FromContext(ctx context.Context) (Viewer, bool) {
	v, ok := ctx.Value(viewerKey).(Viewer)
	return v, ok
}
