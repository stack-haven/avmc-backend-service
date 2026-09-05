# 词法规范文件 (Lexical Spec)

> **Schema**: `evie-tool/lexspec/v1`  
> **Mode**: `production_real`（真实链路 evie-tool 识别结果）  
> **Generated**: 2026-09-05T13:42:38+08:00  
> **Service**: evie-tool v0.1.0  
> **Endpoint**: `POST /evie/tool/v1/asr:recognize`  
> **Transport**: json/base64 (HTTP gateway)

## 1. 输入音频

| 字段 | 值 |
|---|---|
| 路径 | `./testdata/晨会录音.mp3` |
| 字节数 | 621816 |
| 格式 | mp3 |
| Provider | funasr |
| 流式 | false |

## 2. 租户上下文

| 字段 | 值 |
|---|---|
| tenantId | `1889501240003497986` |
| token | `bearer:6dcd5e06...` |
| userId | `u-prod-1` |
| deptId | `1904450235179954177` |

## 3. 词库构成

### 3.1 系统静态词库 (version=2026-09-01)

路径：`./configs/dictionaries/system.json`

| Standard | Category | Prio | Aliases | Corrections | Homophones |
|---|---|---:|---|---|---|
| 金种籽 | PRODUCT | 100 | 金种子, 金种仔 | — | 金中子, 金种种 |
| 指令官 | PRODUCT | 90 | — | — | 纸灵官, 指令关 |
| 技术研发部 | ORGANIZATION | 80 | 研发部, 技术部 | — | — |

#### 系统 phrase_rules

| From | To |
|---|---|
| `个种籽` | `颗种籽` |
| `个种子` | `颗种籽` |

### 3.2 租户词库（qua 同步）

- users endpoint: `/admin-api/qua/member-extended/page?selectAll=true`
- depts endpoint: `/admin-api/system/dept/list`
- users_active: 9
- users_filtered (status=0): 1
- depts_active: 3
- sync log: `synced tenant 1889501240003497986: 73 entries, 65 relations`

#### qua 同步快照（sample，前 20 条）

| ID | Standard | Category | Source | Status |
|---|---|---|---|---|
| 10001 | 佘丽群 | PERSON | qua_user | active |
| 10002 | 周丽群 | PERSON | qua_user | active |
| 10003 | 测试1 | PERSON | qua_user | active |
| 10004 | 王五 | PERSON | qua_user | active |
| 10005 | 赵六 | PERSON | qua_user | active |
| 10006 | 孙七 | PERSON | qua_user | disabled |
| 10007 | 周八 | PERSON | qua_user | active |
| 10008 | 吴九 | PERSON | qua_user | active |
| 10009 | 郑十 | PERSON | qua_user | active |
| 10010 | 钱多多 | PERSON | qua_user | active |
| 101 | 技术研发部 | ORGANIZATION | qua_dept | active |
| 102 | 产品运营部 | ORGANIZATION | qua_dept | active |
| 103 | 金种籽事业部 | ORGANIZATION | qua_dept | active |

### 3.3 总数

| 项 | 数 |
|---|---:|
| system_entries | 3 |
| system_phrase_rules | 2 |
| qua_entries_total | 13 |

## 4. 增强 Pipeline

- engine: `pkg/lexnorm`
- workspace: `go.work (backend-service + pkg/lexnorm)`
- order: `normalize → disfluency → alias → deterministic → pinyin → fuzzy_vocab → ctxproc`

### 4.1 规则阈值

| 规则 | 值 |
|---|---|
| clean_normalize | true |
| pinyin_threshold | 0.85 |
| fuzzy_auto_threshold | 0.80 |
| fuzzy_suggest_threshold | 0.60 |
| fuzzy_person_threshold | 0.65 |
| disfluency_words | 啊, 呃, 哦, 嗯, 那个, 这个 |

## 5. 识别结果

### 5.1 Raw Text（funasr 输出）

字符数：586

```
对一下我天的情况。好呃，昨天的金种子情况呢，夏总给陈欣静加了三十个金种子做标书方案的设计方案，呃，给伍西辉加了二十个金种子整理导航软件信息输入，给伍西辉加五颗黑种子设计门市文化墙，未考虑呃展成问题。好，我们的熊经理给我们的田华加了五颗黑种子，量化客户使用中反馈配置无法生效，确信给杨新宇加了十颗金种子，按照要求完成播种未来的功能开发。给田青加了十五颗金种子验收测试播种未来功能，找出多处的一个错误，给田华加了十颗金种子，参加客户问题客户问题反馈会议快速解决方案啊，我们的田华给田青加了一颗黑种子黑种子的设计图，出现一副企议啊，给杨新宇加了十五颗金种子播种未来主题创建模块开发。给婷青加了十五克金种是总子任务提醒信息呃，交叉设计，给陈科航加了五颗金种子。早上打招呼，好，我们的袁总给田华加了给云，给田华和杨新宇加了二十克金种子，每人帮助搬运花盆，给李宇豪加了十颗k种子管理视频封面出作，给田华招了五颗黑种子。是啊，办公桌面前的植物未浇水，给林玉豪加了三十个斤种子。龚老师的几个账号视频按时完成好，李海燕给朱凤加了二十克金种子，协助查询发票信息，给天华加了二十个金种子，就是其他平台的v i p分享使用周丽群给陈科航加了二十个金种子，协助卸货，给陈新进加了四颗金种子，帮助同事转文件给武锡辉和卢川加了二十个黑种，是每人多次提醒未完成产值的一个表格整理啊，这是昨天的新融织情况。
```

### 5.2 Enhanced Text

字符数：578

```
对一下我天的情况。好,昨天的金种籽情况呢,夏总给陈欣静加了三十个金种籽做标书方案的设计方案,,给伍西辉加了二十个金种籽整理导航软件信息输入,给伍西辉加五颗黑种子设计门市文化墙,未考虑展成问题。好,我们的熊经理给我们的田华加了五颗黑种子,量化客户使用中反馈配置无法生效,确信给杨新宇加了十颗金种籽,按照要求完成播种未来的功能开发。给田青加了十五颗金种籽验收测试1种未来功能,找出多处的一个错误,给田华加了十颗金种籽,参加客户问题客户问题反馈会议快速解决方案,我们的田华给田青加了一颗黑种子黑种子的设计图,出现一副企议,给杨新宇加了十五颗金种籽播种未来主题创建模块开发。给婷青加了十五克金种是总子任务提醒信息,交叉设计,给陈科航加了五颗金种籽。早上打招呼,好,我们的袁总给田华加了给云,给田华和杨新宇加了二十克金种籽,每人帮助搬运花盆,给李宇豪加了十颗k种子管理视频封面出作,给田华招了五颗黑种子。是,办公桌面前的植物未浇水,给林玉豪加了三十个斤种子。龚老师的几个账号视频按时完成好,李海燕给朱凤加了二十克金种籽,协助查询发票信息,给天华加了二十个金种籽,就是其他平台的v i p分享使用佘丽群给陈科航加了二十个金种籽,协助卸货,给陈新进加了四颗金种籽,帮助同事转文件给武锡辉和卢川加了二十个黑种,是每人多次提醒未完成产值的一个表格整理,这是昨天的新融织情况。
```

### 5.3 Changes 全量

| # | Action | From | To | Type | Source | Conf | Reason |
|---:|---|---|---|---|---|---:|---|
| 0 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 1 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 2 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 3 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 4 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 5 | replace | ` ` | ` ` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 6 | replace | ` ` | ` ` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 7 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 8 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 9 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 10 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 11 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 12 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 13 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 14 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 15 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 16 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 17 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 18 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 19 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 20 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 21 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 22 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 23 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 24 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 25 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 26 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 27 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 28 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 29 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 30 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 31 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 32 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 33 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 34 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 35 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 36 | replace | `，` | `,` | CLEAN | normalize | 1.00 | whitespace and full-width normalization |
| 37 | remove | `啊` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 38 | remove | `啊` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 39 | remove | `呃` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 40 | remove | `啊` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 41 | remove | `啊` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 42 | remove | `呃` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 43 | remove | `呃` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 44 | remove | `呃` | `` | FILLER | disfluency | 1.00 | filler word removal |
| 45 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 46 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 47 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 48 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 49 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 50 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 51 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 52 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 53 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 54 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 55 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 56 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 57 | replace | `金种子` | `金种籽` | ALIAS | alias | 1.00 | alias → canonical: 金种子 → 金种籽 |
| 58 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 59 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 60 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 61 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 62 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 63 | replace | `测试播` | `测试1` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "测试播" → "测试1" (dist=1, conf=0.67) |
| 64 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 65 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 66 | suggest | `金种是` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种是" → "金种籽" (dist=1, conf=0.67) |
| 67 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 68 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 69 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 70 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 71 | replace | `周丽群` | `佘丽群` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "周丽群" → "佘丽群" (dist=1, conf=0.67) |
| 72 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |
| 73 | suggest | `金种子` | `金种籽` | FUZZY | fuzzy_vocab | 0.67 | fuzzy: "金种子" → "金种籽" (dist=1, conf=0.67) |

## 6. 统计

### 6.1 总数

- total_changes = **74**

### 6.2 By Action

| Action | Count |
|---|---:|
| remove | 8 |
| replace | 52 |
| suggest | 14 |

### 6.3 By Type

| Type | Count |
|---|---:|
| ALIAS | 13 |
| CLEAN | 37 |
| FILLER | 8 |
| FUZZY | 16 |

### 6.4 By Source

| Source | Count |
|---|---:|
| alias | 13 |
| disfluency | 8 |
| fuzzy_vocab | 16 |
| normalize | 37 |

### 6.5 Alias 命中（按出现次数排序）

| From | To | Count |
|---|---|---:|
| `金种子` | `金种籽` | 13 |

### 6.6 Fuzzy 替换命中

| From | To | Confidence |
|---|---|---:|
| `测试播` | `测试1` | 0.67 |
| `周丽群` | `佘丽群` | 0.67 |

### 6.7 Fuzzy 候选建议（suggest）

| From | To | Confidence |
|---|---|---:|
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种是` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |
| `金种子` | `金种籽` | 0.67 |

### 6.8 高置信度替换（conf ≥ 0.8 且 action=replace）

| Type | From | To | Source | Conf | Count |
|---|---|---|---|---:|---:|
| CLEAN | `，` | `,` | normalize | 1.00 | 35 |
| ALIAS | `金种子` | `金种籽` | alias | 1.00 | 13 |
| CLEAN | ` ` | ` ` | normalize | 1.00 | 2 |

## 7. 一致性（5 次真实请求）

### 7.1 Raw text MD5（funasr 端）

- run 1: `e2d4e159`
- run 2: `75bcc4fc`
- run 3: `6c69edbb`
- run 4: `46ceb06a`
- run 5: `bc2926e3`

### 7.2 Enhanced text MD5（lexnorm 端）

- run 1: `3e8534e8`
- run 2: `90e1f1c8`
- run 3: `4d059bca`
- run 4: `1ddbc678`
- run 5: `e35fec55`

> funasr 输出本身有微抖动（声学模型非完全确定性）；lexnorm pipeline 对相同 rawText 完全确定性。规范以 enhanced md5 中位数为基准。

## 8. 验收 Checklist

- [x] system.json 加载成功（3 条 + 2 phrase_rules）
- [x] qua 同步成功（synced tenant 1889501240003497986: 73 entries, 65 relations）
- [x] Bearer Token 鉴权通过
- [x] funasr ASR 返回（provider=）
- [x] lexnorm Pipeline 跑完 7 步
- [x] 总变更数 = 74
- [x] Alias 命中 ≥ 1：`金种子` → `金种籽`（13 次）
- [x] Fuzzy 替换 ≥ 1：`测试播` → `测试1`（conf=0.67）
