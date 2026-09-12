# 示例索引

> 所有示例位于 `example/`。运行方式见各示例目录内的说明。

| 示例 | 主题 | 说明 |
|---|---|---|
| [01-basic](../example/01-basic) | 引擎入门 | 最小可用 Normalize |
| [02-pipeline](../example/02-pipeline) | 管线组合 | 自定义 Processor 顺序 |
| [03-presets](../example/03-presets) | 预设 | Standard / HighAccuracy / Fast / Transcripts / ScannedText |
| [04-custom-processor](../example/04-custom-processor) | 自定义 Processor | 含能力 Descriptor 声明（DescribedProcessor） |
| [05-hooks](../example/05-hooks) | Hook / Middleware | 横切能力与 panic 恢复 |
| [06-lexicon-store](../example/06-lexicon-store) | 词库热更新 | Store 快照一致性 |
| [07-profiles](../example/07-profiles) | 多 Profile | 业务上下文隔离 |
| [08-error-handling](../example/08-error-handling) | 错误策略 | FailFast / ContinueOnError |
| [09-compare-engine-vs-llm](../example/09-compare-engine-vs-llm) | 引擎 vs LLM | 真实数据对比 + stress test（51 用例）+ 缺陷目录 |

## 关键代码片段

### 独立运行一个 Processor

```go
state, _ := lexnorm.NewState(ctx, text, lex, lexnorm.DefaultConfig())
_ = alias.New(lex).Process(ctx, state)
```

### 组合与调整管线（不重排用户顺序）

```go
p := lexnorm.NewPipeline(normalize.New(), myProc, alias.New(lex))
p2, err := lexnorm.RemoveProcessor(p, "normalize")
```

### 读取能力元数据（分类 / 确定性）

```go
d, declared := lexnorm.DescriptorOf(proc)
fmt.Println(d.Name, d.Category, d.Mutation())
```

### 词库卫生校验（推荐在 Build 前调用）

```go
b := lexicon.NewBuilder().Add(entries...)
if err := b.Validate(); err != nil {
    log.Fatal(err) // 子串 variant / canonical 撞车 / 重复 variant
}
lex, err := b.Build()
```

更完整的可运行示例请进入对应目录；规范细节见
[processor.md](processor.md)。
