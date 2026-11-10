package service

import (
	"context"
	"runtime"
	"time"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
)

// Version 由 main 通过 -ldflags 注入；空串时回退为 dev。
var Version = "dev"

// SystemServiceService 系统信息服务。
type SystemServiceService struct {
	pb.UnimplementedSystemServiceServer
	log       *log.Helper
	startedAt time.Time
	runMode   string
	dataDir   string
	dbDriver  string
	httpAddr  string
}

// SystemOptions 系统信息的只读运行参数。
//
// 用独立结构体而非多个 string 参数，避免 wire 无法区分同类型依赖。
type SystemOptions struct {
	DBDriver string
	HTTPAddr string
}

// NewSystemServiceService 创建系统信息服务。
func NewSystemServiceService(opts SystemOptions, logger log.Logger) *SystemServiceService {
	return &SystemServiceService{
		log:       log.NewHelper(log.With(logger, "module", "edai/service/system")),
		startedAt: time.Now(),
		runMode:   runtimeconfig.RunMode(),
		dataDir:   runtimeconfig.DataDir(),
		dbDriver:  opts.DBDriver,
		httpAddr:  opts.HTTPAddr,
	}
}

// GetSystemInfo 返回只读运行信息。
func (s *SystemServiceService) GetSystemInfo(ctx context.Context, _ *pb.GetSystemInfoRequest) (*pb.GetSystemInfoResponse, error) {
	return &pb.GetSystemInfoResponse{
		Info: &pb.SystemInfo{
			Version:        Version,
			RunMode:        s.runMode,
			DatabaseDriver: s.dbDriver,
			DataDir:        s.dataDir,
			HttpAddr:       s.httpAddr,
			StartedAt:      s.startedAt.Format(time.RFC3339),
		},
	}, nil
}

var _ = runtime.Version
