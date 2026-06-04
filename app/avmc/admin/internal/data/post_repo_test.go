package data

import (
	"io"
	"strings"
	"testing"

	pb "backend-service/api/avmc/admin/v1"
	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestPostRepoListReturnsPersistedFields(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()

	repo := NewPostRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	created, err := repo.Save(ctx, &pbCore.Post{
		Name:   ptr("developer"),
		Sort:   ptr(int32(7)),
		Status: ptr(enum.Status_STATUS_ENABLED),
		Remark: ptr("core team"),
	})
	if err != nil {
		t.Fatalf("save post: %v", err)
	}
	if created.GetSort() != 7 || created.GetStatus() != enum.Status_STATUS_ENABLED || created.GetRemark() != "core team" {
		t.Fatalf("created post = %#v", created)
	}

	posts, err := repo.ListPosts(ctx)
	if err != nil {
		t.Fatalf("list posts: %v", err)
	}
	if len(posts) != 1 || posts[0].GetName() != "developer" || posts[0].GetSort() != 7 ||
		posts[0].GetStatus() != enum.Status_STATUS_ENABLED || posts[0].GetRemark() != "core team" {
		t.Fatalf("posts = %#v", posts)
	}
}

func TestPostRepoReturnsTypedBusinessErrors(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()
	repo := NewPostRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.Save(ctx, &pbCore.Post{}); !pb.IsPostNameCannotBeEmpty(err) {
		t.Fatalf("Save(empty) error = %v", err)
	}
	if _, err := repo.FindByID(ctx, 999); !pb.IsPostNotFound(err) {
		t.Fatalf("FindByID(missing) error = %v", err)
	}
}

func TestPostRepoSavePropagatesUniquenessQueryError(t *testing.T) {
	client := newTestClient(t)
	repo := NewPostRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	_, err := repo.Save(tenantContext(1), &pbCore.Post{Name: ptr("developer")})
	if err == nil || !strings.Contains(err.Error(), "checking post name uniqueness") {
		t.Fatalf("Save() error = %v, want uniqueness query error", err)
	}
}
