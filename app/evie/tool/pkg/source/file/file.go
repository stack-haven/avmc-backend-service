// Package file provides a vocabulary Source adapter that reads
// entities from a local JSON or YAML file. It is intended for:
//   - demo mode (zero external dependencies)
//   - integration tests (golden-file style)
//   - single-tenant deployments with static vocabularies
//
// # File format
//
// The file is a top-level JSON/YAML object:
//
//	{
//	  "version": "v1",
//	  "users": [
//	    {"id": "u1", "name": "Alice", "mobile": "138..."},
//	    ...
//	  ],
//	  "departments": [
//	    {"id": "d1", "name": "Engineering"},
//	    ...
//	  ]
//	}
//
// Each user / department is emitted as a RawEntity whose Data field
// is the row itself.
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"backend-service/app/evie/tool/pkg/source"
)

// Config configures a file-backed Source.
type Config struct {
	// Path is the file to read. Required.
	Path string

	// UserEntityType / DeptEntityType are emitted on RawEntity.EntityType
	// (defaults "user" / "department").
	UserEntityType string
	DeptEntityType string

	// UsersKey / DeptsKey are JSON keys in the root object (defaults
	// "users" / "departments"). The value must be a JSON array.
	UsersKey string
	DeptsKey string

	// ReloadOnFetch makes every Fetch re-read the file from disk
	// (useful for hot-reload during development). Defaults to false
	// (one-shot read at construction).
	ReloadOnFetch bool
}

// Source is a file-backed vocabulary Source.
type Source struct {
	cfg    Config
	cached []source.RawEntity
}

// New constructs a Source by reading the file immediately.
func New(cfg Config) (*Source, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("file: Path required")
	}
	if cfg.UserEntityType == "" {
		cfg.UserEntityType = "user"
	}
	if cfg.DeptEntityType == "" {
		cfg.DeptEntityType = "department"
	}
	if cfg.UsersKey == "" {
		cfg.UsersKey = "users"
	}
	if cfg.DeptsKey == "" {
		cfg.DeptsKey = "departments"
	}
	s := &Source{cfg: cfg}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// Name implements source.Source.
func (s *Source) Name() string { return "file" }

// Fetch returns the entities loaded at construction time, unless
// ReloadOnFetch is true (in which case the file is re-read on every
// call).
func (s *Source) Fetch(_ context.Context) ([]source.RawEntity, error) {
	if s.cfg.ReloadOnFetch {
		if err := s.reload(); err != nil {
			return nil, err
		}
	}
	return s.cached, nil
}

// reload reads the file and caches the parsed entities.
func (s *Source) reload() error {
	abs, err := filepath.Abs(s.cfg.Path)
	if err != nil {
		return fmt.Errorf("file: abs path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("file: read %s: %w", abs, err)
	}
	root, err := decode(data)
	if err != nil {
		return fmt.Errorf("file: decode %s: %w", abs, err)
	}
	out := make([]source.RawEntity, 0)
	if arr, ok := root[s.cfg.UsersKey].([]any); ok {
		for _, v := range arr {
			if m, ok := v.(map[string]any); ok {
				out = append(out, source.RawEntity{
					SourceID:   idOf(m),
					EntityType: s.cfg.UserEntityType,
					Source:     s.Name(),
					Data:       m,
				})
			}
		}
	}
	if arr, ok := root[s.cfg.DeptsKey].([]any); ok {
		for _, v := range arr {
			if m, ok := v.(map[string]any); ok {
				out = append(out, source.RawEntity{
					SourceID:   idOf(m),
					EntityType: s.cfg.DeptEntityType,
					Source:     s.Name(),
					Data:       m,
				})
			}
		}
	}
	s.cached = out
	return nil
}

// decode accepts JSON or YAML-light (we currently support JSON only;
// YAML support is added by importing gopkg.in/yaml.v3 if needed).
func decode(data []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err == nil {
		return root, nil
	}
	return nil, fmt.Errorf("only JSON supported in this build")
}

func idOf(m map[string]any) string {
	for _, k := range []string{"id", "ID", "Id"} {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case string:
				return x
			case float64:
				if x == float64(int64(x)) {
					return fmt.Sprintf("%d", int64(x))
				}
			}
		}
	}
	return ""
}
