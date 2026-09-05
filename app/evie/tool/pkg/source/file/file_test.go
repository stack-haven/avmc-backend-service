package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	filesrc "backend-service/app/evie/tool/pkg/source/file"
)

const sampleJSON = `{
  "version": "v1",
  "users": [
    {"id": "u1", "name": "Alice"},
    {"id": "u2", "name": "Bob"}
  ],
  "departments": [
    {"id": "d1", "name": "Engineering"}
  ]
}`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vocab.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestFileSource_HappyPath(t *testing.T) {
	path := writeTemp(t, sampleJSON)
	src, err := filesrc.New(filesrc.Config{Path: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if src.Name() != "file" {
		t.Errorf("Name = %q, want file", src.Name())
	}

	entities, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(entities) != 3 {
		t.Fatalf("entities = %d, want 3 (2 users + 1 dept)", len(entities))
	}

	var users, depts int
	for _, e := range entities {
		switch e.EntityType {
		case "user":
			users++
			if e.Data["name"] == nil {
				t.Errorf("user missing name: %+v", e.Data)
			}
		case "department":
			depts++
		default:
			t.Errorf("unexpected entity type: %q", e.EntityType)
		}
	}
	if users != 2 {
		t.Errorf("users = %d, want 2", users)
	}
	if depts != 1 {
		t.Errorf("depts = %d, want 1", depts)
	}
}

func TestFileSource_ReloadOnFetch(t *testing.T) {
	path := writeTemp(t, sampleJSON)
	src, _ := filesrc.New(filesrc.Config{Path: path, ReloadOnFetch: true})

	entities, _ := src.Fetch(context.Background())
	if len(entities) != 3 {
		t.Errorf("first fetch: %d entities, want 3", len(entities))
	}

	// Mutate file then re-fetch.
	updated := `{"users":[{"id":"u3","name":"Carol"}],"departments":[]}`
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	entities, _ = src.Fetch(context.Background())
	if len(entities) != 1 || entities[0].SourceID != "u3" {
		t.Errorf("after reload: %+v", entities)
	}
}

func TestFileSource_NoReload(t *testing.T) {
	path := writeTemp(t, sampleJSON)
	src, _ := filesrc.New(filesrc.Config{Path: path}) // ReloadOnFetch defaults to false

	_, _ = src.Fetch(context.Background())
	updated := `{"users":[],"departments":[]}`
	_ = os.WriteFile(path, []byte(updated), 0o644)
	entities, _ := src.Fetch(context.Background())
	if len(entities) != 3 {
		t.Errorf("cached fetch should still return 3, got %d", len(entities))
	}
}

func TestFileSource_EmptyArrays(t *testing.T) {
	path := writeTemp(t, `{"users":[],"departments":[]}`)
	src, _ := filesrc.New(filesrc.Config{Path: path})
	entities, _ := src.Fetch(context.Background())
	if len(entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(entities))
	}
}

func TestFileSource_MissingPath(t *testing.T) {
	_, err := filesrc.New(filesrc.Config{Path: ""})
	if err == nil {
		t.Errorf("expected error for empty path")
	}
}

func TestFileSource_NonExistentPath(t *testing.T) {
	_, err := filesrc.New(filesrc.Config{Path: "/no/such/file.json"})
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}
}

func TestFileSource_BadJSON(t *testing.T) {
	path := writeTemp(t, "{ not valid json")
	_, err := filesrc.New(filesrc.Config{Path: path})
	if err == nil {
		t.Errorf("expected decode error")
	}
}

func TestFileSource_CustomKeys(t *testing.T) {
	custom := `{"members":[{"id":"m1","name":"Alice"}],"teams":[{"id":"t1","name":"Eng"}]}`
	path := writeTemp(t, custom)
	src, err := filesrc.New(filesrc.Config{
		Path:      path,
		UsersKey:  "members",
		DeptsKey:  "teams",
		UserEntityType: "member",
		DeptEntityType: "team",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entities, _ := src.Fetch(context.Background())
	if len(entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(entities))
	}
	if entities[0].EntityType != "member" && entities[0].EntityType != "team" {
		t.Errorf("unexpected entity_type: %s", entities[0].EntityType)
	}
}
