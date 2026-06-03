package data

import (
	"testing"
	"time"

	"backend-service/app/version/service/internal/conf"

	"google.golang.org/protobuf/types/known/durationpb"
)

func TestDatabasePoolDefaults(t *testing.T) {
	if got := databaseMaxIdleConnections(nil); got != 10 {
		t.Fatalf("default max idle = %d", got)
	}
	if got := databaseMaxOpenConnections(nil); got != 50 {
		t.Fatalf("default max open = %d", got)
	}
	if got := databaseConnectionMaxLifetime(nil); got != 30*time.Minute {
		t.Fatalf("default max lifetime = %v", got)
	}

	cfg := &conf.Data{Database: &conf.Data_Database{
		MaxIdleConnections:    4,
		MaxOpenConnections:    8,
		ConnectionMaxLifetime: durationpb.New(2 * time.Minute),
	}}
	if got := databaseMaxIdleConnections(cfg); got != 4 {
		t.Fatalf("max idle = %d", got)
	}
	if got := databaseMaxOpenConnections(cfg); got != 8 {
		t.Fatalf("max open = %d", got)
	}
	if got := databaseConnectionMaxLifetime(cfg); got != 2*time.Minute {
		t.Fatalf("max lifetime = %v", got)
	}
}

func TestConfigDuration(t *testing.T) {
	fallback := time.Second
	if got := configDuration(nil, fallback); got != fallback {
		t.Fatalf("nil duration = %v, want %v", got, fallback)
	}
	if got := configDuration(durationpb.New(0), fallback); got != fallback {
		t.Fatalf("zero duration = %v, want %v", got, fallback)
	}
	if got := configDuration(durationpb.New(250*time.Millisecond), fallback); got != 250*time.Millisecond {
		t.Fatalf("explicit duration = %v", got)
	}
}
