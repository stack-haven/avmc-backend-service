package biz

import (
	"context"

	pb "backend-service/api/version/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrUserNotFound is user not found.
// ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// ReleaseRepo is a Greater repo.
type ReleaseRepo interface {
	Save(context.Context, *pb.Release) (*pb.Release, error)
	Update(context.Context, *pb.Release) (*pb.Release, error)
	FindByID(context.Context, int64) (*pb.Release, error)
	ListByHello(context.Context, string) ([]*pb.Release, error)
	ListAll(context.Context) ([]*pb.Release, error)
}

// ReleaseUsecase is a Release usecase.
type ReleaseUsecase struct {
	repo ReleaseRepo
	log  *log.Helper
}

// NewReleaseUsecase new a Release usecase.
func NewReleaseUsecase(repo ReleaseRepo, logger log.Logger) *ReleaseUsecase {
	return &ReleaseUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreateRelease creates a Release, and returns the new Release.
func (uc *ReleaseUsecase) CreateRelease(ctx context.Context, g *pb.Release) (*pb.Release, error) {
	uc.log.WithContext(ctx).Infof("CreateRelease: %v", g.Name)
	return uc.repo.Save(ctx, g)
}
