# 文本规范增强 · 真实凭据分析

> 2026-09-08，基于用户提供的真实 qua API（78 用户 / 8 部门）对 evie/tool v0.1.0 的端到端优化分析。

## 文件清单

| 文件 | 描述 | 来源 |
|---|---|---|
| `qua_users_78.json` | qua API `/admin-api/qua/member-extended/page` 完整响应（78 用户） | curl 真实拉取 |
| `qua_depts_8.json` | qua API `/admin-api/system/dept/list` 完整响应（8 部门） | curl 真实拉取 |
| `people_dept_summary.json` | 用户名 → 部门映射摘要（精简版） | Python 处理 |
| `enhance_v1_response.json` | evie/tool `/enhance` 接口 v1 响应（修复前 58 处改动） | curl 真实调用 |
| `enhance_v2_response.json` | evie/tool `/enhance` 接口 v2 响应（修复后 58 处改动） | curl 真实调用 |
| `optimization_plan_v2.md` | 完整优化方案文档（含根因分析、v1→v2 对比、未实施项） | 本次整理 |

## 关键发现

| 项 | v1 | v2 |
|---|---|---|
| fuzzy_vocab 准确率 | 60% (6/10) | 60% (6/10)，严重错纠已消除 |
| 严重错纠（conf<0.5） | 1 处（菌种子→金种籽）| 0 处 |
| 错纠（同 Hamming 冲突）| 2 处（田华→田花 x2）| 2 处（待 5.5 修复）|
| 漏纠人名 | 6 个 | 4 个（叶海嫣/夏其军/袁孟莲/阳巡宇 进字典后自动纠）|

## v2 已实施

1. `internal/biz/processor/fuzzy_vocab.go` buildIndex 让 lockAlias entry 不进 byLen bucket
2. `configs/dictionaries/system.json` 所有 PRODUCT 加 lock_alias=true
3. `configs/dictionaries/system.json` 从 qua API 补全 40 个真人名

## 待实施（v3）

1. fuzzy_vocab 同名字段 conflict 解决（`田华` vs `田花`）
2. qua API 自动化字典同步（vocab sync 增量更新）
3. 上下文判断（"高效X" / "主动X" 模式保留原文）

## 复现命令

```bash
# 拉 qua 数据
curl -s --url 'http://api.bdksim-pro.test.bedoke.com/admin-api/system/dept/list' \
  -H 'authorization: Bearer be3e0d79e55f4840a3605406bda0e32a' \
  -H 'tenant-id: 1889501240003497986' \
  -H 'zone: Asia/Shanghai' \
  -o qua_depts_8.json

curl -s --url 'http://api.bdksim-pro.test.bedoke.com/admin-api/qua/member-extended/page?&selectAll=true' \
  -H 'authorization: Bearer be3e0d79e55f4840a3605406bda0e32a' \
  -H 'tenant-id: 1889501240003497986' \
  -H 'zone: Asia/Shanghai' \
  -o qua_users_78.json

# 调 evie/tool enhance（需 Redis token）
curl -X POST 'http://localhost:8110/evie/tool/v1/enhance' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <redis oauth2_access_token>' \
  -d '{"text": "<待增强文本>"}' \
  | tee enhance_response.json
```
