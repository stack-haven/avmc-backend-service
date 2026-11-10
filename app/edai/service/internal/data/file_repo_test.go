package data

import (
	"database/sql"
	"sync"
	"testing"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/data/ent/gen"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"

	_ "github.com/glebarez/go-sqlite"
)

// openTestData 构造一个内存库的 Data，用于仓储层单测。
//
// 用纯 Go 驱动（glebarez/go-sqlite）+ memory 模式：不落盘、不需要 cgo，
// 与 EDAI 生产配置同驱动，避免测试与运行时行为不一致。
//
// 注意：该驱动的注册名是 "sqlite"，而 Ent 的 dialect 名只有 "sqlite3"，
// 因此必须像生产代码那样经 database/sql 打开后再 entsql.OpenDB 包一层。
func openTestData(t *testing.T) *Data {
	t.Helper()
	db, err := sql.Open("sqlite", "file:edai_repo_test?mode=memory&cache=shared&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// cache=shared 保证多个连接看到同一个内存库；连接数要 >1，
	// 否则并发用例会被连接池串行化，测不出真实竞争。
	db.SetMaxOpenConns(8)
	client := gen.NewClient(gen.Driver(entsql.OpenDB("sqlite3", db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	d, cleanup, err := NewData(client, FixedTenantResolver{Tenant: 1}, log.NewStdLogger(discard{}))
	if err != nil {
		t.Fatalf("new data: %v", err)
	}
	t.Cleanup(cleanup)
	return d
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func newRecord(sourceID uint32, path string) *pb.FileRecord {
	ext := ".txt"
	size := int64(10)
	return &pb.FileRecord{
		SourceId:         sourceID,
		RawPath:          path,
		RawParentPath:    "/tmp",
		FileName:         "f.txt",
		Extension:        &ext,
		FileSize:         size,
		FileStatus:       "present",
		QuickFingerprint: "10-1",
		FirstSeenAt:      "2026-01-01T00:00:00Z",
		LastScannedAt:    "2026-01-01T00:00:00Z",
	}
}

// TestUpsertScanned_ConcurrentSamePath 覆盖并发扫描同一路径的竞争场景。
//
// 背景（QA 实测缺陷）：UpsertScanned 是「先查再写」。两个扫描任务同时处理同一路径时
// 双方都可能查不到而同时 INSERT，后者撞唯一约束 → 调用方计 failed →
// 该路径不进入 ScannedPaths → 全量扫描随后把它误标 missing。
//
// 修复后要求：并发写同一路径必须全部成功、只留一条记录、状态为 present。
func TestUpsertScanned_ConcurrentSamePath(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)
	const path = "/tmp/a.txt"

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			if _, err := repo.UpsertScanned(t.Context(), newRecord(1, path)); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent upsert failed: %v", err)
	}

	ctx := d.TenantContext(t.Context())
	n, err := d.Ent().FileRecord.Query().Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 record, got %d", n)
	}

	row, err := d.Ent().FileRecord.Query().Only(ctx)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	status := row.FileStatus
	if status != "present" {
		t.Fatalf("expected status present, got %s", status)
	}
}

// TestUpsertScanned_RepeatedIsIdempotent 校验重复写入只更新不新增。
func TestUpsertScanned_RepeatedIsIdempotent(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)
	const path = "/tmp/b.txt"

	created, err := repo.UpsertScanned(t.Context(), newRecord(1, path))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !created {
		t.Fatal("first upsert should create")
	}

	created, err = repo.UpsertScanned(t.Context(), newRecord(1, path))
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if created {
		t.Fatal("second upsert should not create")
	}

	n, err := d.Ent().FileRecord.Query().Count(d.TenantContext(t.Context()))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 record, got %d", n)
	}
}
