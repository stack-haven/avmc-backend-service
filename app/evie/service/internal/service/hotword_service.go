package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// HotwordServiceService 热词管理服务。
type HotwordServiceService struct {
	pb.UnimplementedHotwordServiceServer
	huc *biz.HotwordUsecase
	log *log.Helper
}

// NewHotwordServiceService 创建热词服务实例。
func NewHotwordServiceService(huc *biz.HotwordUsecase, logger log.Logger) *HotwordServiceService {
	return &HotwordServiceService{huc: huc, log: log.NewHelper(logger)}
}

// ListHotwords 查询热词列表。
func (s *HotwordServiceService) ListHotwords(ctx context.Context, req *pb.ListHotwordsRequest) (*pb.ListHotwordsResponse, error) {
	hotwords, err := s.huc.List(ctx, req.GetCategory())
	if err != nil {
		return nil, err
	}
	return &pb.ListHotwordsResponse{Hotwords: hotwords}, nil
}

// UpsertHotword 新增或更新热词。
func (s *HotwordServiceService) UpsertHotword(ctx context.Context, req *pb.UpsertHotwordRequest) (*pb.UpsertHotwordResponse, error) {
	hotword := &pb.Hotword{
		Id:       req.GetId(),
		Word:     req.GetWord(),
		Target:   req.GetTarget(),
		Weight:   req.GetWeight(),
		Category: req.GetCategory(),
	}
	upserted, err := s.huc.Upsert(ctx, hotword)
	if err != nil {
		return nil, err
	}
	return &pb.UpsertHotwordResponse{Hotword: upserted}, nil
}

// DeleteHotword 删除热词。
func (s *HotwordServiceService) DeleteHotword(ctx context.Context, req *pb.DeleteHotwordRequest) (*pb.DeleteHotwordResponse, error) {
	if err := s.huc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteHotwordResponse{}, nil
}
