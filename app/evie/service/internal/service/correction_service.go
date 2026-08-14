package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// CorrectionServiceService 智能纠错引擎服务。
type CorrectionServiceService struct {
	pb.UnimplementedCorrectionServiceServer
	engine *biz.CorrectionEngine
	log    *log.Helper
}

// NewCorrectionServiceService 创建纠错服务实例。
func NewCorrectionServiceService(engine *biz.CorrectionEngine, logger log.Logger) *CorrectionServiceService {
	return &CorrectionServiceService{engine: engine, log: log.NewHelper(logger)}
}

// Correct 文本纠错。
func (s *CorrectionServiceService) Correct(ctx context.Context, req *pb.CorrectRequest) (*pb.CorrectResponse, error) {
	return s.engine.Correct(ctx, req)
}
