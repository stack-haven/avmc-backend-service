package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// EnhancementLogRepo 增强记录仓库接口。
type EnhancementLogRepo interface {
	List(ctx context.Context, req *pb.ListEnhancementLogsRequest) ([]*pb.EnhancementLog, int32, error)
	Get(ctx context.Context, id uint32) (*pb.EnhancementLog, error)
	Save(ctx context.Context, log *EnhancementLogData) error
}

// EnhancementLogData 增强记录写入数据（业务模型）。
type EnhancementLogData struct {
	RequestID           string
	SessionID           string
	PolicyID            uint32
	PolicyMode          string
	ContextVersion      string
	RawText             string
	EnhancedText        string
	ChangesJSON         string
	ProcessingTimeMs    int64
	CleaningTimeMs      int64
	FillerTimeMs        int64
	VocabMatchTimeMs    int64
	AliasTimeMs         int64
	DeterministicTimeMs int64
	PinyinTimeMs        int64
	FuzzyTimeMs         int64
	ContextTimeMs       int64
	Status              int32
	ErrorMessage        string
}

// EnhancementLogUsecase 增强记录业务逻辑。
type EnhancementLogUsecase struct {
	repo EnhancementLogRepo
	log  *log.Helper
}

// NewEnhancementLogUsecase 创建增强记录 usecase。
func NewEnhancementLogUsecase(repo EnhancementLogRepo, logger log.Logger) *EnhancementLogUsecase {
	return &EnhancementLogUsecase{repo: repo, log: log.NewHelper(logger)}
}

// List 查询增强记录列表。
func (uc *EnhancementLogUsecase) List(ctx context.Context, req *pb.ListEnhancementLogsRequest) ([]*pb.EnhancementLog, int32, error) {
	return uc.repo.List(ctx, req)
}

// Get 查询增强记录详情。
func (uc *EnhancementLogUsecase) Get(ctx context.Context, id uint32) (*pb.EnhancementLog, error) {
	return uc.repo.Get(ctx, id)
}

// Save 保存增强记录（错误降级：保存失败不阻断文本增强主流程）。
func (uc *EnhancementLogUsecase) Save(ctx context.Context, data *EnhancementLogData) error {
	if data == nil || data.RawText == "" {
		return nil
	}
	if err := uc.repo.Save(ctx, data); err != nil {
		uc.log.Warnf("save enhancement log failed: %v", err)
	}
	return nil
}
