# pkg/textenhance 开源化计划

## 目标

把 `pkg/textenhance` 单独抽出，做成可独立演进、规范扩展的开源文本增强引擎。

## 当前状态

- ✅ 9 个 processor + Pipeline + 3 层 HA 保护
- ✅ 业务无关（不依赖 ASR/qua/Redis）
- ✅ 确定性输出（跨进程/跨 sync）
- ❌ Configuration 硬编码（不能租户覆盖）
- ❌ 没有 plugin 协议（第三方扩展困难）
- ❌ 没有 benchmark / 文档站点
- ❌ 业务数据混在 system.json
- ❌ changelog / CONTRIBUTING / LICENSE 缺失

## 下一轮新对话

**主题**：pkg/textenhance 开源化设计

**讨论重点**：
1. **目录重命名**：`pkg/textenhance` → `pkg/textengine`？（与"文本增强"语义对齐）
2. **架构拆分**：
   - `core/` — 框架（Processor / Pipeline / Snapshot）
   - `processors/` — 内置策略
   - `plugin/` — 第三方扩展协议
   - `config/` — 多租户配置加载
3. **Plugin 协议设计**：Go interface + YAML manifest + wasm/lua 二级扩展？
4. **配置系统**：当前硬编码阈值 → YAML/JSON 配置 + per-tenant override + 热加载
5. **数据模型解耦**：拆开 system.json（业务词库）与 framework config（系统参数）
6. **Benchmark + 文档**：godoc + example + 性能基线
7. **License / README / CHANGELOG / CONTRIBUTING**

**重要原则**：
- 开源版本**不依赖** evie/tool 的任何代码（不 import biz/data）
- 词库是输入数据，不是框架的一部分
- 错误信息中英文双份
- API 稳定性（v1.0 锁定接口）

## 工作流

1. 新对话中先确认需求边界（哪些 processor 进开源、哪些留 evie/tool）
2. 拆 6-8 个 milestone（每个 1-2 周）
3. 每个 milestone 单独 PR + tag
