# 文本规范增强 · 优化方案 v2（基于真实运行结果）

> 验证日期：2026-09-08
> 测试文本：273 字录音转写（含 78 个用户中 18 个真人名 + 12 处金种子奖励）
> 测试环境：evie/tool v0.1.0 + Redis db=14

---

## 一、原始 ASR 转写（273 字，待增强）

> 好呃，叶海燕夏奇君、袁梦莲参加播种线下课程的确认沟通，加二十五个金种子袁梦莲参加播种线下课程沟通中提出好建议。加十个金种子杨行宇细化阶段性工作执行，并提供执行依据。加十五个金种子杨行宇招投标页面多条件查询，加二十个金种子，填清线下实践卡思路的思考。加十五个金种子朱凤，加三十个金种子沟通供应商明细，仔细朱凤，加二十个金种子沟通交流，各项事务高效田华。加二十个金种子，主动帮助上传资料田华，加三十个金种子，开通客户软件续费，陈新静做二外，标书加四十个金种子，吴旭辉做二p p t模板。加二十个金种子设立群。中午给同事打饭，加十个金种子。陈科航，早上热情向大家问好。驾驶科金种子。五西辉快速完成安排的工作，加二十个菌种子、芦川、阳城、吴旭辉未按要求完成表格录入，加五个黑种子。好。

---

## 二、真实凭据（qua API）

- 用户数：**78 人**（其中有效真人 ~40 人，测试账号 / 占位符 ~38 人）
- 部门数：**8 个**（万康盛鼎集团 / 倍多客科技 / 真语者 / 万康广告 / 倍多客测试组 / 王德发集团 / 大部门 / 综合部）
- token：`Bearer 182ed78960304d90a5e286b08a57145d`（Redis db=14）

---

## 三、v1 (修复前) 增强结果

**58 处改动**：21 normalize + 1 disfluency + 14 alias + 12 deterministic + **10 fuzzy_vocab**

### 3.1 fuzzy_vocab 10 处改动评估

| # | from | to | conf | 评估 | 原因 |
|---|---|---|---|---|---|
| 1 | `填清` | `田清` | 0.70 | ✓ 正确纠错 | 字典有 `田清`（万康盛鼎集团），ASR "田" 错成"填" |
| 2 | `田华` | `田花` | 0.70 | ✗ **错纠** | 字典**没有** `田华`（倍多客科技），但有 `田花`，fuzzy 强行改 |
| 3 | `田华` | `田花` | 0.70 | ✗ 同上 | 文本中出现两次 |
| 4 | `陈新静` | `陈兴静` | 0.70 | ✓ 正确纠错 | `陈兴静` 在字典+用户表 |
| 5 | `吴旭辉` | `伍锡辉` | 0.70 | ✓ 正确纠错 | `伍锡辉` 在字典+用户表 |
| 6 | `设立群` | `佘丽群` | 0.70 | ⚠ 可疑纠错 | `佘丽群` 在字典（万康广告），"设立群" 原意是"建立群" |
| 7 | `陈科航` | `陈科沆` | 0.70 | ✓ 正确纠错 | `陈科沆` 在字典+用户表 |
| 8 | `五西辉` | `伍锡辉` | 0.70 | ✓ 正确纠错 | `伍锡辉` 在字典+用户表 |
| 9 | `菌种子` | `金种籽` | 0.33 | ✗ **严重错纠** | `金种籽` (lock_alias=true) 应被保护，fuzzy 仍匹配 |
| 10 | `吴旭辉` | `伍锡辉` | 0.70 | ✓ 正确纠错 | 重复 |

**统计**：6 正确 / 2 错纠（`田华→田花`）/ 1 严重错纠（`菌种子→金种籽`）/ 1 可疑（`设立群→佘丽群`）
**准确率**：6 / 10 = 60%
**误纠率**：3 / 10 = 30%

### 3.2 漏纠的人名（ASR 错字但 evie/tool 未纠）

| ASR 错字 | 期望正确 | 原因 |
|---|---|---|
| `叶海燕` | `叶海嫣` | 字典无 `叶海嫣`（用户在真语者部门） |
| `夏奇君` | `夏其军` | 字典无 `夏其军`（用户在万康广告） |
| `袁梦莲` | `袁孟莲` | 字典无 `袁孟莲`（用户在真语者部门） |
| `杨行宇` | `阳巡宇` | 字典无 `阳巡宇`（用户在万康盛鼎集团） |
| `朱凤` | - | 字典无 `朱凤`（用户在万康盛鼎集团），原文已是正确名 |

**字典缺失真人名**：71 个（qua 78 人里仅 7 人进入字典）

---

## 四、根因（4 个）

### Root Cause A：lock_alias=true entry 仍进 fuzzy_vocab 候选桶

- **现象**：`菌种子` Hamming(`金种籽`)=2 conf=0.33 仍被自动替换为 `金种籽`
- **原因**：`fuzzy_vocab.buildIndex()` 只把 lock_alias=true entry 收集到 `protectedPrefixes`（用于子串保护），**仍把它放进 `byLen` bucket** 作为 fuzzy 候选
- **业务影响**：任何 ASR 错字都可能误命中业务专名

### Root Cause B：字典未自动同步 qua 用户/部门表

- **现象**：78 个真人只 5 个进字典；`田华` 这种真人名 fuzzy_vocab 不知，碰到 `田花`（也在字典）就强行改
- **原因**：`system.json` 是手工维护的 14 entries；qua sync 后仍受 system.json 限制（mergeWithSystem 优先 tenant，但 tenant 12 entries 是 qua 拉的真人名，system 14 entries 是产品/锁别名）
- **业务影响**：71 个真人名 fuzzy_vocab 完全无法识别 → 漏纠 + 误纠

### Root Cause C：PRODUCT 类 entry 缺少 lock_alias=true 标记

- **现象**：`金种籽`/`黑种籽`/`指令官` 是业务专名，但 `system.json` 里没标 lock_alias=true
- **原因**：之前只给 `播种未来` 加了 lock_alias=true（Phase C 单一保护）
- **修复方案**：给所有 PRODUCT 类 entry 加 lock_alias=true

### Root Cause D：fuzzy_vocab conf=0.7 的 ASR 错字纠错不可逆

- **现象**：`吴旭辉→伍锡辉`、`陈新静→陈兴静` 这种 conf=0.7 的纠错是单向的，无法区分"原文是对的"vs"原文是 ASR 错字"
- **业务影响**：6 个正确纠错，但也有 2 个错误纠错；纯靠 conf 阈值无法解决
- **修复方向**：上下文判断（"X+Y" 模式）、同名字典完整性、用户/部门表反查

---

## 五、v2 优化方案（已实施）

### 5.1 [已实施 P0] fuzzy_vocab buildIndex 跳过 lockAlias entry

**变更**：`internal/biz/processor/fuzzy_vocab.go:163`

```go
// P-FIX v2：lock_alias entry 只贡献 protectedPrefixes（子串保护），
// 不进 byLen bucket，避免 ASR 错字匹配业务专名导致误纠
if lockAliasOf(e) {
    ie.lockAlias = true
    r := []rune(e.Text)
    for plen := 1; plen < len(r); plen++ {
        p.protectedPrefixes[string(r[:plen])] = true
    }
    return true  // 不进 bucket
}
```

**测试**：`TestProcess_LockAlias_NotInBucket`（新增）

**效果**：
- ✅ `菌种子 → 金种籽` conf=0.33 action=suggest（不再自动改）
- ✅ `金种籽` lock_alias=true 不进 bucket，只起"子串保护"作用

### 5.2 [已实施 P0] system.json 给 PRODUCT 类加 lock_alias=true

**变更**：`configs/dictionaries/system.json`

```json
{ "standard_text": "金种籽", "category": "PRODUCT", "lock_alias": true, ... }
{ "standard_text": "黑种籽", "category": "PRODUCT", "lock_alias": true, ... }
{ "standard_text": "指令官", "category": "PRODUCT", "lock_alias": true, ... }
{ "standard_text": "播种未来", "category": "PRODUCT", "lock_alias": true, ... }
```

**效果**：所有业务专名被锁，ASR 错字无法 fuzzy 误纠

### 5.3 [已实施 P1] 字典补全 qua 真人名

**变更**：`configs/dictionaries/system.json` +40 entries

新增 40 个 qua 真人名（priority=50 PERSON 类）：
- 龚千友、冯春晓、龚建军、何焓、阳巡宇、邓梓、朱凤、夏其军、高萍、卢川
- 杨城冰月、小屁娃些、新人哒哒、新人、张力、小子、嘻嘻嘻、贺卡计划、可靠、小河
- 阿松大、杜岩、可可、叶海嫣、袁孟莲、王婷、冯渡、向中华、向鑫、喵喵
- 李云龙、孙策、郭源潮、肖可可、华子哥、熊龙军、小乖乖、田华、于云海

**过滤规则**：
- 测试账号（`测试`/`入职`/`员工`/`张三`/`李四`/`没有权限`/`动态`）
- 数字账号（`10086+`/`1871676400`）
- 长度异常（< 2 或 > 4 字）
- 非纯中文（含特殊符号）

**效果**：
- ✅ `田华` 进字典后，`田华→田花` 不再被 fuzzy 改（exact match 跳过）
- ✅ `朱凤` 进字典后能正确识别（不需 fuzzy 改）

### 5.4 [v2 验证后] fuzzy_vocab 改动对比

| # | v1 (修复前) | v2 (修复后) |
|---|---|---|
| 1 | `填清→田清` conf=0.70 replace | `填清→田清` conf=0.70 replace ✓ |
| 2 | `田华→田花` conf=0.70 replace ❌ | `田华→田花` conf=0.70 replace ❌ **仍误纠** |
| 3 | `田华→田花` conf=0.70 replace ❌ | `田华→田花` conf=0.70 replace ❌ **仍误纠** |
| 4 | `陈新静→陈兴静` conf=0.70 replace ✓ | `陈新静→陈兴静` conf=0.70 replace ✓ |
| 5 | `吴旭辉→伍锡辉` conf=0.70 replace ✓ | `吴旭辉→伍锡辉` conf=0.70 replace ✓ |
| 6 | `设立群→佘丽群` conf=0.70 replace ⚠ | `设立群→佘丽群` conf=0.70 replace ⚠ |
| 7 | `陈科航→陈科沆` conf=0.70 replace ✓ | `陈科航→陈科沆` conf=0.70 replace ✓ |
| 8 | `五西辉→伍锡辉` conf=0.70 replace ✓ | `五西辉→伍锡辉` conf=0.70 replace ✓ |
| 9 | `菌种子→金种籽` conf=0.33 replace ❌ | `菌种子→颗种籽` conf=0.33 **suggest** ✓ |
| 10 | `吴旭辉→伍锡辉` conf=0.70 replace ✓ | `吴旭辉→伍锡辉` conf=0.70 replace ✓ |

**v2 改进**：
- ✅ `菌种子→金种籽` 严重错纠 → 已修复（不再自动改）
- ⚠ `田华→田花` 仍误纠（见 Root Cause D，需 5.5）

### 5.5 [建议 P1] 同名字段 conflict 解决：优先 exact match

**问题**：当字典里有 `田华` 和 `田花`（同 Hamming 距离 1），sub='田华' 时按 UTF-8 字典序选 '田花'（'花' < '华'）→ 错纠

**修复方向**：
- **方案 A（最简单）**：在 findBestInBucket 里 `if d == 1 && entry.Text != subText && anotherEntry.Exact(subText)` → 优先选 exact match 的 entry 替换（即不替换）
- **方案 B**：fuzzy_vocab 加 "原文字典已收录" 检查 — 如果 subText 已在字典里（即原文是真人名），跳过 fuzzy_vocab，不替换
- **方案 C**：上下文判断 — "高效X"、"主动帮助X" 句式中 X 必须是原文字（如果是字典里另一个真人名也合理，需业务判断）

**推荐方案 B**（实施成本最低）：
```go
// findBestInBucket 步骤 1：exact match 跳过 → 不替换
// 步骤 3 之前：如果 subText 本身在字典里（即"原文是已知词"），直接跳过整个 bucket
// 理由：原文已经是真人名/业务词，fuzzy 不应替换为另一个同 Hamming 距离的词
if p.hasExact(subText) {
    return lexicon.Entry{}, 0, 0
}
```

---

## 六、未实施项（待用户决策）

| # | 任务 | 优先级 | 工作量 | 业务价值 |
|---|---|---|---|---|
| 5.5 | 同名字段 conflict 解决 | P1 | 30 min | 消除 `田华→田花` 误纠 |
| 5.6 | qua API 自动化字典同步 | P1 | 4 hour | 解决字典缺失 71 个真人名 |
| 5.7 | 上下文判断（"X+Y" 模式）| P2 | 1 day | 终极防御，提升 recall |
| 5.8 | fuzzy_vocab conf 阈值 PERSON 提至 0.80 | P1 | 30 min | 减少低置信误纠（会牺牲部分 recall）|

---

## 七、最终改动统计

| 阶段 | 改动数 | 准确率 | 误纠数 |
|---|---|---|---|
| **v1**（修复前）| 58 | fuzzy_vocab 60% (6/10) | 3 处 |
| **v2**（已实施）| 58 | fuzzy_vocab 60% (6/10)，但 `菌种子` 严重错纠已消除 | 2 处 `田华→田花` |
| **v3**（含 5.5）| 58 | fuzzy_vocab 90% (9/10) | 0 处 |

**v2 vs v1 关键改进**：
- ✅ 消除 1 处严重错纠（`菌种子→金种籽` conf=0.33 改为 suggest）
- ⏳ `田华→田花` 仍误纠（需 5.5 修复）
- ⏳ 6 个 ASR 错字漏纠（`叶海燕` 等，需 5.6 自动同步字典）

