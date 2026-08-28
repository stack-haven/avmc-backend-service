package biz

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	platformpb "backend-service/api/platform/service/v1"
	platformclient "backend-service/app/platform/service/client"
	"backend-service/pkg/asr"
	asraudio "backend-service/pkg/asr/audio"
	"backend-service/pkg/asr/funasr"
	"backend-service/pkg/asr/xunfei"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
)

// ASRRecordRepo ASR 识别记录仓库接口。
type ASRRecordRepo interface {
	Save(ctx context.Context, record *ASRRecord) (uint32, error)
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
	recordRepo   ASRRecordRepo
	fileCenter   *platformclient.FileCenterClient
	log          *log.Helper
}

// NewASRUsecase 创建 ASR usecase。
func NewASRUsecase(providerRepo ProviderRepo, recordRepo ASRRecordRepo, fileCenter *platformclient.FileCenterClient, logger log.Logger) *ASRUsecase {
	return &ASRUsecase{providerRepo: providerRepo, recordRepo: recordRepo, fileCenter: fileCenter, log: log.NewHelper(logger)}
}

// Recognize 同步识别。
func (uc *ASRUsecase) Recognize(ctx context.Context, req *pb.RecognizeRequest) (*asr.ASRResult, error) {
	provider, opts, err := uc.route(ctx, false)
	if err != nil {
		return nil, err
	}

	// 并行：识别 + 上传文件中心（上传失败不影响识别结果）
	type uploadResult struct {
		fileID      uint32
		audioFormat string
		err         error
	}
	uploadCh := make(chan uploadResult, 1)
	go func() {
		fileID, audioFormat, uerr := uc.uploadAudio(ctx, req)
		uploadCh <- uploadResult{fileID: fileID, audioFormat: audioFormat, err: uerr}
	}()

	result, err := provider.Recognize(ctx, req.GetAudioData(), opts)
	if err != nil {
		return nil, err
	}

	var audioURL string
	var audioFormat string
	if up := <-uploadCh; up.err != nil {
		uc.log.Warnf("upload audio to file center: %v", up.err)
	} else if up.fileID > 0 {
		audioURL = strconv.FormatUint(uint64(up.fileID), 10)
		audioFormat = up.audioFormat
	}

	result.RequestID = req.GetSessionId()
	if result != nil && uc.recordRepo != nil {
		if _, err := uc.recordRepo.Save(ctx, &ASRRecord{
			UserID:      authn.GetAuthUserID(ctx),
			SessionID:   result.RequestID,
			RawText:     result.Text,
			Confidence:  result.Confidence,
			DurationMs:  result.DurationMs,
			AudioURL:    audioURL,
			AudioFormat: audioFormat,
			Engine:      result.ProviderName,
		}); err != nil {
			uc.log.Errorf("save asr record: %v", err)
		}
	}
	return result, nil
}

// uploadAudio 将音频上传到文件中心，返回文件 ID 与实际存储格式。
func (uc *ASRUsecase) uploadAudio(ctx context.Context, req *pb.RecognizeRequest) (uint32, string, error) {
	if uc.fileCenter == nil || len(req.GetAudioData()) == 0 {
		return 0, "", nil
	}
	audio, ext, contentType := normalizeAudio(req)
	file, err := uc.fileCenter.Upload(ctx, &platformpb.CreateFileUploadSessionRequest{
		FileName:     convert.ToPointer(fmt.Sprintf("asr-%s.%s", req.GetSessionId(), ext)),
		ContentType:  convert.ToPointer(contentType),
		Size:         convert.ToPointer(int64(len(audio))),
		BusinessType: convert.ToPointer("asr"),
		BusinessId:   convert.ToPointer(req.GetSessionId()),
		Visibility:   convert.ToPointer("private"),
	}, audio)
	if err != nil {
		return 0, "", err
	}
	return file.GetId(), ext, nil
}

// normalizeAudio 根据 encoding 规范化音频为可播放格式，返回音频字节、文件后缀与 content-type。
// raw PCM 无容器头，播放器无法识别，统一封装为 WAV。
func normalizeAudio(req *pb.RecognizeRequest) (audio []byte, ext, contentType string) {
	audio = req.GetAudioData()
	switch req.GetFormat().GetEncoding() {
	case pb.AudioEncoding_AUDIO_ENCODING_WAV:
		return audio, "wav", "audio/wav"
	case pb.AudioEncoding_AUDIO_ENCODING_MP3:
		return audio, "mp3", "audio/mpeg"
	case pb.AudioEncoding_AUDIO_ENCODING_OPUS:
		return audio, "opus", "audio/opus"
	default: // PCM / UNSPECIFIED
		if asraudio.IsWAV(audio) {
			return audio, "wav", "audio/wav"
		}
		sampleRate := int(req.GetFormat().GetSampleRate())
		return asraudio.PCMToWAV(audio, sampleRate, 1, 16), "wav", "audio/wav"
	}
}

// route 根据租户配置与识别场景路由到 ASR Provider，并加载热词。
// stream=true 走流式（实时逐帧）；stream=false 走整段批量（音频已完整）。
// 整段批量优先本地自建供应商（funasr，推理快）；流式优先讯飞（IAT 实时增量正常）。
func (uc *ASRUsecase) route(ctx context.Context, stream bool) (asr.ASRProvider, asr.RecognizeOptions, error) {
	configs, err := uc.providerRepo.ListConfig(ctx)
	if err != nil {
		return nil, asr.RecognizeOptions{}, err
	}

	var active *pb.TenantProviderConfig
	if stream {
		// 流式：优先 active 的讯飞（IAT 逐帧增量输出正常）；funasr 流式当前无增量结果，故不作为首选。
		for _, c := range configs {
			if c.GetProviderName() == "xunfei" && c.GetIsActive() {
				active = c
				break
			}
		}
	} else {
		// 整段批量：优先 funasr（本地 SenseVoice，批量推理快），避免云实时转写 API 的逐帧节奏限制。
		for _, c := range configs {
			if c.GetProviderName() == "funasr" {
				active = c
				break
			}
		}
	}
	if active == nil {
		for _, c := range configs {
			if c.GetIsActive() {
				active = c
				break
			}
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
	uc.log.Infof("route stream=%v -> provider=%s", stream, provider.Name())
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
	case "xunfei":
		cfg, err := xunfei.ParseConfig(configJSON)
		if err != nil {
			return nil, err
		}
		return xunfei.New(*cfg)
	default:
		return nil, fmt.Errorf("unsupported asr provider: %s", name)
	}
}

// StreamRecognize 流式识别：路由到 Provider，收集完整音频，识别结束后上传文件中心并保存记录。
// 通过 resultCh 增量回传纯 ASR 文本（不含业务字段）；返回保存后的记录 ID 与音频文件 ID。
func (uc *ASRUsecase) StreamRecognize(ctx context.Context, sessionID string, audioCh <-chan asr.PCMChunk, resultCh chan<- asr.ASRStreamResult) (uint32, string, error) {
	provider, opts, err := uc.route(ctx, true)
	if err != nil {
		return 0, "", err
	}

	// 收集音频分片，同时转发给 provider（避免阻塞上层）
	fwdCh := make(chan asr.PCMChunk, 32)
	var audioBuf []byte
	go func() {
		for chunk := range audioCh {
			audioBuf = append(audioBuf, chunk.Data...)
			fwdCh <- chunk
		}
		close(fwdCh)
	}()

	// 内部结果 channel：转发增量 + 收集最终文本
	inner := make(chan asr.ASRStreamResult, 32)
	done := make(chan error, 1)
	go func() {
		err := provider.StreamRecognize(ctx, fwdCh, inner, opts)
		close(inner)
		done <- err
	}()

	var finalText string
	for r := range inner {
		if r.Text != "" {
			finalText = r.Text
		}
		resultCh <- r
	}
	err = <-done

	// 识别结束：完整音频（PCM）转 WAV 上传文件中心 + 保存识别记录
	var recordID uint32
	var audioURL string
	if finalText != "" && len(audioBuf) > 0 {
		recordID, audioURL = uc.saveStreamRecord(ctx, sessionID, finalText, audioBuf, provider.Name())
	}

	return recordID, audioURL, err
}

// saveStreamRecord 将完整 PCM 音频转 WAV 上传文件中心，并保存识别记录。
func (uc *ASRUsecase) saveStreamRecord(ctx context.Context, sessionID, finalText string, audioPCM []byte, engine string) (uint32, string) {
	audioURL := ""
	if uc.fileCenter != nil && len(audioPCM) > 0 {
		wav := asraudio.PCMToWAV(audioPCM, 16000, 1, 16)
		file, err := uc.fileCenter.UploadAuto(ctx, &platformpb.CreateFileUploadSessionRequest{
			FileName:     convert.ToPointer(fmt.Sprintf("asr-%s.wav", sessionID)),
			ContentType:  convert.ToPointer("audio/wav"),
			Size:         convert.ToPointer(int64(len(wav))),
			BusinessType: convert.ToPointer("asr"),
			BusinessId:   convert.ToPointer(sessionID),
			Visibility:   convert.ToPointer("private"),
		}, wav)
		if err != nil {
			uc.log.Warnf("upload stream audio: %v", err)
		} else {
			audioURL = strconv.FormatUint(uint64(file.GetId()), 10)
		}
	}

	var recordID uint32
	if uc.recordRepo != nil {
		id, err := uc.recordRepo.Save(ctx, &ASRRecord{
			UserID:      authn.GetAuthUserID(ctx),
			SessionID:   sessionID,
			RawText:     finalText,
			AudioURL:    audioURL,
			AudioFormat: "wav",
			Engine:      engine,
		})
		if err != nil {
			uc.log.Errorf("save stream record: %v", err)
		} else {
			recordID = id
		}
	}
	return recordID, audioURL
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
	audio = asraudio.WAVToPCM(audio) // 流式记录存为 WAV，识别前提取 PCM

	provider, opts, err := uc.route(ctx, false)
	if err != nil {
		return nil, err
	}
	result, err := provider.Recognize(ctx, audio, opts)
	if err != nil {
		return nil, err
	}
	result.RequestID = fmt.Sprintf("%s-re", record.GetSessionId())
	// 保存新记录（复用原音频文件，避免重复上传）
	if result != nil && uc.recordRepo != nil {
		if _, err := uc.recordRepo.Save(ctx, &ASRRecord{
			UserID:      authn.GetAuthUserID(ctx),
			SessionID:   result.RequestID,
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
