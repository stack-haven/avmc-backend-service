# ark-lexnorm · Claude Code 入口

> Claude Code 进入本目录时**第一份必读**。
> 详细项目指南见 `.agents/AGENTS.md`，本文件仅作精简入口。

## 快速开始

1. **读 `.agents/AGENTS.md`** — 项目入口与中断恢复协议
2. **读 `.agents/RULES.md`** — 命名硬约束 + 15 条不变量 + 决策日志（D1-D7）
3. **读 `.agents/REVIEW.md`** — Code Review 检查清单（**任一阻断项不通过即拒绝合并**）
4. **读 `docs/README.md`** — 18 份拆解文档的索引与阅读路径

## 当前阶段

**文档阶段已完成，代码实施待启动。** 当前 M：M1（项目骨架）。

详见 `docs/17-开发实施路线.md` §6。

## 4 个核心决策（**D1-D4**，必记）

| ID | 决策 | 错误示范 |
|:--:|---|---|
| **D1** | LLM 不在 Standard Preset | `preset.Standard()` 含 `processor/llm` |
| **D2** | `New(...) (*Engine, error)` | `func New(...) *Engine` |
| **D3** | Result 保留 Original/Duration/Steps/Err | 删 `Original` / `Duration` 字段 |
| **D4** | Match 冲突：Longest → Priority → Lex | 任意改变顺序 |

## 命名硬约束

核心包**禁止**出现：`tenant` / `ASR` / `OCR` / `HR` / `Employee` / `Customer` / `Meeting` / `Agent` / `Document` / `产品`。

如需表达"业务上下文隔离" → `Profile`。
如需表达"知识来源" → `LexiconSource`。

## 依赖约束

```bash
go list -deps ./... | grep -v "^github.com/stack-haven/lexnorm" | grep -v "^internal/"
```

**只允许依赖标准库。**

## 验证命令

```bash
# 必跑
go test ./...
go test -race ./...
go vet ./...

# M12 阶段必须达标
go test -bench=. -benchmem ./...
go test -fuzz=FuzzXxx ./...
```

## 完整路径

```
.agents/AGENTS.md        → 项目入口
.agents/RULES.md         → 规则
.agents/DESIGN.md        → 设计原则
.agents/REVIEW.md        → Review 清单
docs/README.md           → 文档索引
docs/00~17-*.md          → 拆解文档
docs/ark-lexnorm-架构设计与开发规范1.2.md  → 权威规范
```
