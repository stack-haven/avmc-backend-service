package data

import (
	"testing"
	"time"

	"backend-service/app/ai/service/internal/conf"

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
		MaxIdleConnections:    3,
		MaxOpenConnections:    7,
		ConnectionMaxLifetime: durationpb.New(time.Minute),
	}}
	if got := databaseMaxIdleConnections(cfg); got != 3 {
		t.Fatalf("max idle = %d", got)
	}
	if got := databaseMaxOpenConnections(cfg); got != 7 {
		t.Fatalf("max open = %d", got)
	}
	if got := databaseConnectionMaxLifetime(cfg); got != time.Minute {
		t.Fatalf("max lifetime = %v", got)
	}
}
