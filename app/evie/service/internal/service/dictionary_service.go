package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// DictionaryServiceService 字典中心服务。
type DictionaryServiceService struct {
	pb.UnimplementedDictionaryServiceServer
	duc *biz.DictionaryUsecase
	log *log.Helper
}

// NewDictionaryServiceService 创建字典中心服务实例。
func NewDictionaryServiceService(duc *biz.DictionaryUsecase, logger log.Logger) *DictionaryServiceService {
	return &DictionaryServiceService{duc: duc, log: log.NewHelper(logger)}
}

// ListWords 分页查询标准词。
func (s *DictionaryServiceService) ListWords(ctx context.Context, req *pb.ListWordsRequest) (*pb.ListWordsResponse, error) {
	words, total, err := s.duc.ListWords(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListWordsResponse{Words: words, Total: total}, nil
}

// GetWord 查询标准词详情。
func (s *DictionaryServiceService) GetWord(ctx context.Context, req *pb.GetWordRequest) (*pb.GetWordResponse, error) {
	word, err := s.duc.GetWord(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetWordResponse{Word: word}, nil
}

// CreateWord 创建标准词。
func (s *DictionaryServiceService) CreateWord(ctx context.Context, req *pb.CreateWordRequest) (*pb.CreateWordResponse, error) {
	word := &pb.DictionaryWord{
		Word:     req.GetWord(),
		Category: req.GetCategory(),
		Level:    req.GetLevel(),
		Priority: req.GetPriority(),
		Aliases:  req.GetAliases(),
	}
	created, err := s.duc.CreateWord(ctx, word)
	if err != nil {
		return nil, err
	}
	return &pb.CreateWordResponse{Word: created}, nil
}

// UpdateWord 更新标准词。
func (s *DictionaryServiceService) UpdateWord(ctx context.Context, req *pb.UpdateWordRequest) (*pb.UpdateWordResponse, error) {
	word := &pb.DictionaryWord{
		Id:       req.GetId(),
		Word:     req.GetWord(),
		Category: req.GetCategory(),
		Priority: req.GetPriority(),
		Status:   req.GetStatus(),
	}
	updated, err := s.duc.UpdateWord(ctx, word)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateWordResponse{Word: updated}, nil
}

// DeleteWord 删除标准词。
func (s *DictionaryServiceService) DeleteWord(ctx context.Context, req *pb.DeleteWordRequest) (*pb.DeleteWordResponse, error) {
	if err := s.duc.DeleteWord(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteWordResponse{}, nil
}
