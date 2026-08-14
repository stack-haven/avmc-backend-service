package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/pkg/asr"
	"backend-service/pkg/asr/funasr"
	"backend-service/pkg/auth/authn"
)

// ASRRecordRepo ASR 识别记录仓库接口。
type ASRRecordRepo interface {
	Save(ctx context.Context, record *ASRRecord) error
	List(ctx context.Context, req *pb.ListAsrRecordsRequest) ([]*pb.AsrRecord, int32, error)
	Get(ctx context.Context, id uint32) (*pb.AsrRecord, error)
}

// ASRRecord ASR 识别记录（业务模型）。
type ASRRecord struct {
	UserID          uint32
	SessionID       string
	RawText         string
	Confidence      float64
	DurationMs      int64
	AudioDurationMs int
	AudioURL        string
	AudioFormat     string
	Engine          string
}

// ASRUsecase ASR 语音识别编排。
type ASRUsecase struct {
	providerRepo ProviderRepo
	hotwordRepo  HotwordRepo
	recordRepo   ASRRecordRepo
	log          *log.Helper
}

// NewASRUsecase 创建 ASR usecase。
func NewASRUsecase(providerRepo ProviderRepo, hotwordRepo HotwordRepo, recordRepo ASRRecordRepo, logger log.Logger) *ASRUsecase {
	return &ASRUsecase{providerRepo: providerRepo, hotwordRepo: hotwordRepo, recordRepo: recordRepo, log: log.NewHelper(logger)}
}

// Recognize 同步识别。
func (uc *ASRUsecase) Recognize(ctx context.Context, req *pb.RecognizeRequest) (*asr.ASRResult, error) {
	provider, opts, err := uc.route(ctx)
	if err != nil {
		return nil, err
	}
	result, err := provider.Recognize(ctx, req.GetAudioData(), opts)
	if err != nil {
		return nil, err
	}
	if result != nil && uc.recordRepo != nil {
		if err := uc.recordRepo.Save(ctx, &ASRRecord{
			UserID:      authn.GetAuthUserID(ctx),
			SessionID:   req.GetSessionId(),
			RawText:     result.Text,
			Confidence:  result.Confidence,
			DurationMs:  result.DurationMs,
			AudioFormat: formatEncodingString(req.GetFormat().GetEncoding()),
			Engine:      result.ProviderName,
		}); err != nil {
			uc.log.Errorf("save asr record: %v", err)
		}
	}
	return result, nil
}

// route 根据租户配置路由到 ASR Provider，并加载热词。
func (uc *ASRUsecase) route(ctx context.Context) (asr.ASRProvider, asr.RecognizeOptions, error) {
	configs, err := uc.providerRepo.ListConfig(ctx)
	if err != nil {
		return nil, asr.RecognizeOptions{}, err
	}
	var active *pb.TenantProviderConfig
	for _, c := range configs {
		if c.GetIsActive() {
			active = c
			break
		}
	}
	if active == nil {
		return nil, asr.RecognizeOptions{}, pb.ErrorServiceUnavailable("ASR Provider 尚未启用，请在供应商管理页面启用")
	}

	provider, err := newProvider(active.GetProviderName(), active.GetConfigJson())
	if err != nil {
		return nil, asr.RecognizeOptions{}, pb.ErrorServiceUnavailable("%s", err.Error())
	}

	opts := asr.RecognizeOptions{
		SampleRate: int(active.GetSampleRate()),
		Language:   active.GetLanguage(),
	}
	if provider.Capabilities().HotwordSupport && uc.hotwordRepo != nil {
		if hotwords, err := uc.hotwordRepo.List(ctx, ""); err == nil {
			opts.Hotwords = toAsrHotwords(hotwords)
		}
	}
	return provider, opts, nil
}

// newProvider 根据供应商名与租户配置创建 ASR Provider。
// TODO(B2): 接入 whisper/xunfei/aliyun 时在此扩展工厂分支。
func newProvider(name, configJSON string) (asr.ASRProvider, error) {
	switch name {
	case "funasr":
		cfg, err := funasr.ParseConfig(configJSON)
		if err != nil {
			return nil, err
		}
		return funasr.New(*cfg)
	default:
		return nil, fmt.Errorf("unsupported asr provider: %s", name)
	}
}

// toAsrHotwords 将 proto 热词转为 asr.Hotword。
func toAsrHotwords(hotwords []*pb.Hotword) []asr.Hotword {
	result := make([]asr.Hotword, 0, len(hotwords))
	for _, h := range hotwords {
		result = append(result, asr.Hotword{
			Word:   h.GetWord(),
			Target: h.GetTarget(),
			Weight: float64(h.GetWeight()),
		})
	}
	return result
}

// StreamRecognize 流式识别：路由到 Provider 并回传增量结果。
func (uc *ASRUsecase) StreamRecognize(ctx context.Context, audioCh <-chan asr.PCMChunk, resultCh chan<- asr.ASRStreamResult) error {
	provider, _, err := uc.route(ctx)
	if err != nil {
		return err
	}
	return provider.StreamRecognize(ctx, audioCh, resultCh, asr.RecognizeOptions{})
}

// ListRecords 查询识别记录列表。
func (uc *ASRUsecase) ListRecords(ctx context.Context, req *pb.ListAsrRecordsRequest) ([]*pb.AsrRecord, int32, error) {
	return uc.recordRepo.List(ctx, req)
}

// GetRecord 查询识别记录详情。
func (uc *ASRUsecase) GetRecord(ctx context.Context, id uint32) (*pb.AsrRecord, error) {
	return uc.recordRepo.Get(ctx, id)
}

func formatEncodingString(enc pb.AudioEncoding) string {
	switch enc {
	case pb.AudioEncoding_AUDIO_ENCODING_WAV:
		return "wav"
	case pb.AudioEncoding_AUDIO_ENCODING_MP3:
		return "mp3"
	case pb.AudioEncoding_AUDIO_ENCODING_OPUS:
		return "opus"
	default:
		return "pcm"
	}
}
