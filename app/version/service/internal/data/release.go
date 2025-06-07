package data

import (
	"context"

	"backend-service/app/version/service/internal/biz"

	pb "backend-service/api/version/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

type releaseRepo struct {
	data *Data
	log  *log.Helper
}

// NewReleaseRepo .
func NewReleaseRepo(data *Data, logger log.Logger) biz.ReleaseRepo {
	return &releaseRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *releaseRepo) Save(ctx context.Context, g *pb.Release) (*pb.Release, error) {
	return g, nil
}

func (r *releaseRepo) Update(ctx context.Context, g *pb.Release) (*pb.Release, error) {
	return g, nil
}

func (r *releaseRepo) FindByID(context.Context, int64) (*pb.Release, error) {
	return nil, nil
}

func (r *releaseRepo) ListByHello(context.Context, string) ([]*pb.Release, error) {
	return nil, nil
}

func (r *releaseRepo) ListAll(context.Context) ([]*pb.Release, error) {
	return nil, nil
}
