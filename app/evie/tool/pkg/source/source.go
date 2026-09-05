// Package source provides pluggable VocabularySource adapters for
// pulling raw entities (users, departments, ...) from external
// systems into a downstream normalizer.
//
// # Design goals
//
//   - Business-system agnostic: HTTP/JSON implementations are
//     configurable, not hard-coded to any product.
//   - Composable: every adapter implements the same Source interface
//     so the rest of the pipeline does not know whether vocabulary
//     comes from a remote API, a local file, or a constant table.
//   - Testable: each adapter can be exercised in isolation with
//     httptest, golden files, or in-memory state.
//
// # Built-in adapters
//
//   - evie/tool/pkg/source  : generic REST adapter (any system with JSON APIs)
//   - evie/tool/pkg/source  : JSON / YAML file adapter (offline / demo mode)
//   - evie/tool/pkg/source/qua   : qua-specific adapter (kept as reference impl)
//
// # Adding a new adapter
//
//  1. Implement Source.Fetch(ctx) returning []RawEntity.
//  2. Register a factory in Factory by source name.
//  3. Document the YAML configuration the factory expects.
package source

import (
	"context"
	"fmt"
)

// RawEntity is the unit of data that flows from any Source into the
// downstream Normalizer. Adapters MUST emit entities whose Source
// field is the implementation name (e.g. "http", "file", "qua") and
// whose EntityType matches the value the Normalizer's rules expect
// (typically "user" and "department").
//
// The Data field is opaque: the adapter does not interpret it, and
// the Normalizer's FieldMapper decides how to extract values.
type RawEntity struct {
	// SourceID is a stable identifier within the upstream system.
	// For qua it is the row's "id"; for HTTP it is the configured
	// JSON key (default: "id").
	SourceID string

	// EntityType classifies the entity. The Normalizer matches this
	// against its rule set (e.g. entity_type: "user").
	EntityType string

	// Source is the implementation name (returned by Source.Name()).
	// Useful for the Normalizer to apply source-specific rules.
	Source string

	// Data is the raw payload (typically a map[string]any parsed from
	// JSON). The Normalizer walks this tree via dotted paths.
	Data map[string]any
}

// Source is implemented by every vocabulary adapter.
//
// Implementations MUST be safe for concurrent use by multiple
// goroutines. Fetch may be called from a background ticker as well
// as from a request-path "cache miss" hot path.
type Source interface {
	// Name returns the adapter name (e.g. "http", "file", "qua").
	// Used for diagnostics and the Factory registry.
	Name() string

	// Fetch retrieves all currently-known entities from the backing
	// system. Partial failures (e.g. user endpoint ok, dept endpoint
	// 5xx) MUST return (entities, err) where entities contains the
	// successful subset; only when no data is retrievable should
	// entities be nil.
	Fetch(ctx context.Context) ([]RawEntity, error)
}

// Factory builds a Source from a configuration blob.
//
// Each adapter registers its factory by name. Constructors accept
// a map[string]any (typically parsed from YAML) so they remain
// decoupled from any specific config loader.
type Factory func(cfg map[string]any) (Source, error)

// registry maps source names to factories.
var registry = map[string]Factory{}

// Register makes a factory available by name. Repeated registration
// is a programmer error and panics.
func Register(name string, f Factory) {
	if name == "" {
		panic("source: empty name")
	}
	if f == nil {
		panic(fmt.Sprintf("source: nil factory for %q", name))
	}
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("source: %q already registered", name))
	}
	registry[name] = f
}

// Build constructs a Source by name.
func Build(name string, cfg map[string]any) (Source, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("source: %q not registered (known: %v)", name, Names())
	}
	return f(cfg)
}

// Names returns the registered source names in arbitrary order.
func Names() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
