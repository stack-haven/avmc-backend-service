# pkg/asr

语音识别（ASR）供应商抽象层，以「供应商可替换、可组合」为核心。

## 设计

```text
pkg/asr
├── provider.go    # ASRProvider 接口 + 公共类型（结果/参数/能力声明）
├── registry.go    # 并发安全的供应商注册中心
├── audio/         # PCM/WAV 互转等音频格式工具（各实现与业务层复用）
├── funasr/        # FunASR 实现（本地部署，SenseVoice 批量 + paraformer 流式）
└── xunfei/        # 讯飞 IAT 实现（云 API，实时流式转写）
```

### 核心契约

所有 ASR 引擎实现只需满足 `ASRProvider` 接口：

```go
type ASRProvider interface {
    Name() string
    Recognize(ctx, audio []byte, opts RecognizeOptions) (*ASRResult, error)
    StreamRecognize(ctx, audioCh <-chan PCMChunk, resultCh chan<- ASRStreamResult, opts RecognizeOptions) error
    Capabilities() ProviderCapabilities
}
```

### 能力声明

`ProviderCapabilities` 声明每个供应商的能力边界（是否流式、支持格式、采样率、
部署方式等），供调用方在路由时决策，也用于 UI 展示。能力应如实声明——
例如 FunASR 的 `Streaming` 取决于是否配置了流式服务地址。

## 快速开始

```go
import "backend-service/pkg/asr"

// 1. 注册供应商
reg := asr.NewProviderRegistry()
reg.Register(funasr.New(funasr.Config{Addr: "http://localhost:18000"}))

// 2. 按名称路由
p, err := reg.Get("funasr")
if err != nil { /* handle */ }

// 3. 同步识别
result, err := p.Recognize(ctx, pcm, asr.RecognizeOptions{SampleRate: 16000})
```

详见 [example_test.go](example_test.go)。

## 实现新供应商

1. 在子目录新建包，实现 `ASRProvider` 接口。
2. 提供 `New(cfg)` 构造器与 `ParseConfig(configJSON)` 配置解析。
3. 能力声明如实填写（`Streaming`、`SupportedFormat`、`DeploymentMode` 等）。
4. 音频格式转换复用 `pkg/asr/audio`，不要重复实现。

## 约定

- 供应商包之间相互独立，仅依赖 `pkg/asr` 定义的接口与类型。
- `RecognizeOptions` 字段均为可选，零值表示「未指定、使用默认」。
- 流式实现需保证 `resultCh` 最终发送 `IsFinal` 结果或由调用方感知结束。
- 音频格式：`Recognize` 的 `audio` 为原始字节，格式由各实现解释（参考 `opts.SampleRate`）。
