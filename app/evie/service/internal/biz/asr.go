package biz

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	corepb "backend-service/api/core/service/v1"
	pb "backend-service/api/evie/service/v1"
	"backend-service/pkg/asr"
	"backend-service/pkg/asr/funasr"
	"backend-service/pkg/auth/authn"
	filecentergrpc "backend-service/pkg/filecenter/grpc"
	"backend-service/pkg/utils/convert"
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
	AudioURL        string // 文件中心文件ID（字符串）
	AudioFormat     string
	Engine          string
}

// ASRUsecase ASR 语音识别编排。
type ASRUsecase struct {
	providerRepo ProviderRepo
	hotwordRepo  HotwordRepo
	recordRepo   ASRRecordRepo
	fileCenter   *filecentergrpc.Client
	log          *log.Helper
}

// NewASRUsecase 创建 ASR usecase。
func NewASRUsecase(providerRepo ProviderRepo, hotwordRepo HotwordRepo, recordRepo ASRRecordRepo, fileCenter *filecentergrpc.Client, logger log.Logger) *ASRUsecase {
	return &ASRUsecase{providerRepo: providerRepo, hotwordRepo: hotwordRepo, recordRepo: recordRepo, fileCenter: fileCenter, log: log.NewHelper(logger)}
}

// Recognize 同步识别。
func (uc *ASRUsecase) Recognize(ctx context.Context, req *pb.RecognizeRequest) (*asr.ASRResult, error) {
	provider, opts, err := uc.route(ctx)
	if err != nil {
		return nil, err
	}

	// 并行：识别 + 上传文件中心（上传失败不影响识别结果）
	type uploadResult struct {
		fileID uint32
		err    error
	}
	uploadCh := make(chan uploadResult, 1)
	go func() {
		fileID, uerr := uc.uploadAudio(ctx, req)
		uploadCh <- uploadResult{fileID: fileID, err: uerr}
	}()

	result, err := provider.Recognize(ctx, req.GetAudioData(), opts)
	if err != nil {
		return nil, err
	}

	var audioURL string
	if up := <-uploadCh; up.err != nil {
		uc.log.Warnf("upload audio to file center: %v", up.err)
	} else if up.fileID > 0 {
		audioURL = strconv.FormatUint(uint64(up.fileID), 10)
	}

	if result != nil && uc.recordRepo != nil {
		if err := uc.recordRepo.Save(ctx, &ASRRecord{
			UserID:      authn.GetAuthUserID(ctx),
			SessionID:   req.GetSessionId(),
			RawText:     result.Text,
			Confidence:  result.Confidence,
			DurationMs:  result.DurationMs,
			AudioURL:    audioURL,
			AudioFormat: formatEncodingString(req.GetFormat().GetEncoding()),
			Engine:      result.ProviderName,
		}); err != nil {
			uc.log.Errorf("save asr record: %v", err)
		}
	}
	return result, nil
}

// uploadAudio 将音频上传到文件中心，返回文件 ID。
func (uc *ASRUsecase) uploadAudio(ctx context.Context, req *pb.RecognizeRequest) (uint32, error) {
	if uc.fileCenter == nil || len(req.GetAudioData()) == 0 {
		return 0, nil
	}
	file, err := uc.fileCenter.Upload(ctx, &corepb.CreateFileUploadSessionRequest{
		FileName:     convert.ToPointer(fmt.Sprintf("asr-%s.webm", req.GetSessionId())),
		ContentType:  convert.ToPointer("audio/webm"),
		Size:         convert.ToPointer(int64(len(req.GetAudioData()))),
		BusinessType: convert.ToPointer("asr"),
		BusinessId:   convert.ToPointer(req.GetSessionId()),
		Visibility:   convert.ToPointer("private"),
	}, req.GetAudioData())
	if err != nil {
		return 0, err
	}
	return file.GetId(), nil
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

// ReRecognize 对已有记录重新识别：复用文件中心音频，不重复上传。
func (uc *ASRUsecase) ReRecognize(ctx context.Context, id uint32) (*asr.ASRResult, error) {
	record, err := uc.recordRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.GetAudioUrl() == "" {
		return nil, pb.ErrorResourceNotFound("该记录无原始音频，无法重新识别")
	}
	if uc.fileCenter == nil {
		return nil, pb.ErrorResourceNotFound("文件中心未配置")
	}
	fileID, err := strconv.ParseUint(record.GetAudioUrl(), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("parse audio file id: %w", err)
	}
	audio, _, err := uc.fileCenter.DownloadContent(ctx, uint32(fileID))
	if err != nil {
		return nil, fmt.Errorf("download audio: %w", err)
	}

	provider, opts, err := uc.route(ctx)
	if err != nil {
		return nil, err
	}
	result, err := provider.Recognize(ctx, audio, opts)
	if err != nil {
		return nil, err
	}

	// 保存新记录（复用原音频文件，避免重复上传）
	if result != nil && uc.recordRepo != nil {
		if err := uc.recordRepo.Save(ctx, &ASRRecord{
			UserID:      authn.GetAuthUserID(ctx),
			SessionID:   fmt.Sprintf("%s-re", record.GetSessionId()),
			RawText:     result.Text,
			Confidence:  result.Confidence,
			DurationMs:  result.DurationMs,
			AudioURL:    record.GetAudioUrl(),
			AudioFormat: record.GetAudioFormat(),
			Engine:      result.ProviderName,
		}); err != nil {
			uc.log.Errorf("save re-recognize record: %v", err)
		}
	}
	return result, nil
}

// GetRecordAudio 获取识别记录音频内容（通过文件中心代理下载）。
func (uc *ASRUsecase) GetRecordAudio(ctx context.Context, id uint32) ([]byte, string, error) {
	record, err := uc.recordRepo.Get(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if record.GetAudioUrl() == "" {
		return nil, "", pb.ErrorResourceNotFound("该记录无原始音频")
	}
	if uc.fileCenter == nil {
		return nil, "", pb.ErrorResourceNotFound("文件中心未配置")
	}
	fileID, err := strconv.ParseUint(record.GetAudioUrl(), 10, 32)
	if err != nil {
		return nil, "", fmt.Errorf("parse audio file id: %w", err)
	}
	return uc.fileCenter.DownloadContent(ctx, uint32(fileID))
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
