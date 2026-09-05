# 06 · Lexicon 与热更新

> 源节：§11 Lexicon · §12 LexiconSource · §13 Lexicon Composition · §14 Lexicon Builder · §15 Lexicon Store · §34 高可用架构
> 适用阶段：Phase 4
> 受众：核心开发者
> 关键性：Lexicon 是规范的**知识源**，其构建/查询路径决定运行时性能

---

## 1. Lexicon 定位

Lexicon 表示规范知识。

### 核心实体

```text
Entry
Variant
Relation
Rule
Policy
```

### 边界

**Lexicon 不负责**：

- 数据库存储
- 同步
- 权限
- API
- 多租户
- 生命周期

### 硬约束

**不要在 Lexicon API 中引入具体业务概念**（人名 / 部门 / 商品……）。

---

## 2. Lexicon 接口

```go
package lexicon

type Lexicon interface {
    Entry(id EntryID) (Entry, bool)
    Lookup(text string) (Entry, bool)
    Relations(text string) []Relation
    All(func(Entry) bool)
    Matcher() *Matcher
    Len() int
    Version() string
}
```

### 方法语义

| 方法 | 用途 | 复杂度目标 |
|---|---|---|
| `Entry(id)` | ID 索引，按 ID 取 Entry | O(1) |
| `Lookup(text)` | 按文本取 Entry | O(1) 期望（哈希） |
| `Relations(text)` | 取文本的所有关联（Alias/Deterministic/Homophone 等） | O(1)~O(k) |
| `All(yield)` | 遍历所有 Entry，**确定性顺序** | O(N) |
| `Matcher()` | 返回预构建的 Aho-Corasick 自动机 | O(1) |
| `Len()` | Entry 数量 | O(1) |
| `Version()` | Lexicon 版本号（用于确定性与变更检测） | O(1) |

> **EntryID → Entry 必须是 O(1)**，否则 Alias Resolve 等路径会出现 O(E) 退化。

---

## 3. Entry / Variant / Relation

### Entry

```go
type Entry struct {
    ID       EntryID
    Text     string         // Canonical form
    Class    string         // 类别（开放字符串）
    Variants []Variant      // 该 Entry 的所有变体
    Meta     map[string]any // 任意元数据（无业务约束）
}
```

### Variant

```go
type Variant struct {
    Text       string
    Kind       VariantKind
    Confidence float64       // 0.0 ~ 1.0
    Source     string        // 1.2 决议: string（1.0 是枚举）
}
```

### VariantKind

```go
type VariantKind uint8

const (
    VariantAlias VariantKind = iota        // 别名（同义）
    VariantCorrection                       // 确定性纠错（错字）
    VariantHomophone                        // 同音
    VariantApproximate                      // 近似（编辑距离命中）
)
```

### Relation

```go
type Relation struct {
    From   EntryID
    To     EntryID
    Kind   VariantKind
    Weight float64       // 权重，可用于决策分级
}
```

---

## 4. LexiconSource（**1.1 新增 / 1.2 采纳**）

LexiconSource 是 Lexicon 知识的**来源抽象**，业务系统用自己的 Adapter 把外部数据转换为 LexiconSource。

```go
type LexiconSource interface {
    Version() string
    Entries(yield func(Entry) bool)
    Relations(yield func(Relation) bool)
}
```

### 来源分类

```text
平台标准数据     → LexiconSource
业务系统数据     → LexiconSource（由业务 Adapter 实现）
用户配置数据     → LexiconSource
外部系统同步数据 → LexiconSource
```

**核心包不定义具体业务名称。** 业务层把 HR / CRM / ERP 等映射为 LexiconSource。

### 自定义 LexiconSource 示例

```go
// 用户仓库内
type HRSynonymSource struct {
    api   HRClient
    cache []Entry
}

func (s *HRSynonymSource) Version() string {
    return s.api.LexiconVersion()
}

func (s *HRSynonymSource) Entries(yield func(Entry) bool) {
    for _, e := range s.cache {
        if !yield(e) {
            return
        }
    }
}

func (s *HRSynonymSource) Relations(yield func(Relation) bool) {
    for _, r := range s.relations {
        if !yield(r) {
            return
        }
    }
}
```

> **架构不变量 13**：外部 Lexicon 数据必须经过 Builder 才能进入运行时。

---

## 5. Lexicon Composition（**1.1 新增 / 1.2 采纳**）

多个 LexiconSource 可以组合成一个 Lexicon。

```go
func Compose(sources ...LexiconSource) (Lexicon, error)
```

### 组合流程

```text
Sources
 ↓
Entry Merge
 ↓
Relation Merge
 ↓
冲突检查
 ↓
ID Index
 ↓
Exact Index
 ↓
Aho-Corasick
 ↓
Pinyin Index
 ↓
Fuzzy Index
 ↓
Immutable Snapshot
```

### 冲突必须显式处理

```text
Source A:
小田 → 田华

Source B:
小田 → 田强
```

**Builder 必须返回 `ErrConflict`，或依据明确的优先级策略处理。**

> **禁止静默覆盖**（架构不变量 14：Lexicon 更新必须原子发布）。

---

## 6. Lexicon Builder

```go
type Builder struct {
    // private
}
```

### Build 阶段一次性完成

```text
Entry ID Index          // EntryID → Entry 的 O(1) map
Lookup Index            // Text → Entry 的哈希
Aho-Corasick Matcher    // 全部 Variants 的多模式匹配
n-gram Index            // Fuzzy 候选剪枝
Relation Index          // Variant → 关联 Entry
Deterministic Ordering  // All() 的顺序固定
```

### Build 后语义

> **Runtime Read Only.**

构建完成的 Lexicon 在运行期间不得被修改。

---

## 7. Lexicon Snapshot 与不可变

Lexicon 必须支持**不可变 Snapshot**。

### 运行期

```text
Request
   ↓
Lexicon Snapshot
   ↓
Read Only
```

### 为什么

- 同一 Engine 跨请求共享同一 Lexicon 引用
- 任何请求在执行期间看到的 Lexicon 都是某一刻的完整快照
- 保证确定性

---

## 8. 热更新与 Store

### Store 实现

```go
type Store struct {
    current atomic.Pointer[Lexicon]
}

func (s *Store) Current() *Lexicon {
    return s.current.Load()
}

func (s *Store) Swap(newLex Lexicon) {
    s.current.Store(&newLex)
}
```

### 完整更新流程（**1.1 加强**）

```text
External Source
      ↓
Build
      ↓
Validate
      ↓
New Lexicon Snapshot
      ↓
Atomic Swap
      ↓
New Requests → V2
Old Requests → V1
```

### 关键约束（**架构不变量 14 / 15**）

| # | 约束 | 说明 |
|:--:|---|---|
| 1 | 构建失败不能影响当前版本 | 保留 V1 继续服务 |
| 2 | 校验失败不能替换 | Build 阶段返回 error，V1 保留 |
| 3 | 替换必须原子完成 | `atomic.Pointer.Store` 单步完成 |
| 4 | 正在执行的请求继续使用旧 Snapshot | Lexicon 不可变实现 |
| 5 | 新请求使用新 Snapshot | `Current()` 取当前指针 |
| 6 | **不允许请求中途切换 Snapshot** | Lexicon Runtime 只读（不变量 5） |

---

## 9. 高可用 Lexicon（**1.1 §40.2**）

### Lexicon HA 模型

```text
Immutable Snapshot
+
Atomic Swap
```

### 行为

```text
外部数据源不可用时：
  ↓
当前 Snapshot 继续提供服务

新版本构建失败：
  ↓
V2 Build Failed
  ↓
继续使用 V1
```

> **不得因为词法数据更新失败导致整个规范化服务不可用。**

### Last Known Good 模式

```go
type Store struct {
    current    atomic.Pointer[Lexicon]
    lastKnown  atomic.Pointer[Lexicon]  // 构建失败时的回退
    building   atomic.Bool              // 防止并发构建
}

func (s *Store) TryUpdate(build func() (Lexicon, error)) error {
    if !s.building.CompareAndSwap(false, true) {
        return ErrBuildInProgress
    }
    defer s.building.Store(false)

    newLex, err := build()
    if err != nil {
        return err  // 不替换，V1 继续服务
    }
    s.lastKnown.Store(&newLex)
    s.current.Store(&newLex)
    return nil
}
```

---

## 10. Request Consistency（**1.1 §40.3**）

```text
请求开始：
  Resolve Runtime V1
   ↓
State
   ↓
Processor A
   ↓
Processor B
   ↓
Processor C

即使此时发布 Lexicon V2：
  本请求仍使用 V1

下一请求才使用 V2
```

> **架构不变量 8**：一次请求必须使用一致 Runtime Snapshot。

---

## 11. Build 优化

| 优化 | Build 时 | Runtime 时 |
|---|---|---|
| Aho-Corasick | ✅ 构建 Trie + Failure Link | O(N + Matches) 单次扫描 |
| n-gram Index | ✅ 构建倒排 | 候选剪枝 > 95% |
| Length Filter | ✅ 索引中保存长度 | 长度差过滤 |
| ID Map | ✅ EntryID → Entry | O(1) Resolve |
| 确定性顺序 | ✅ 字典序固化 | All() 顺序稳定 |

### 禁止

- ❌ 对整个 Lexicon 进行无条件暴力匹配
- ❌ Runtime 阶段重新排序 Entry
- ❌ Runtime 阶段构建索引

---

## 12. Lexicon 与 Profile / Runtime 的关系

```text
ProfileResolver
    │ Resolve(ProfileID)
    ▼
Runtime {
    Profile        ── 标识
    Lexicon        ── 知识（来自 LexiconSource 组合）
    Pipeline       ── 流程
    Config         ── 配置
    LexiconVersion ── 快照版本
}
```

一次 Normalize 调用锁定 Runtime 后，Lexicon 引用在整个调用期间不变。

---

## 13. 单元测试要求

- `Lexicon.Entry(id)` 必须覆盖：存在 / 不存在
- `Lexicon.Lookup(text)` 必须覆盖：精确命中 / 大小写 / 完全不命中
- `Lexicon.All()` 多次调用必须返回**相同顺序**
- `Lexicon.Version()` 必须返回非空
- `Build()` 必须返回错误当：重复 ID / 自引用 Relation / 空 Variants
- `Compose(S1, S2)` 在 S1 与 S2 冲突时必须返回 `ErrConflict`
- `Store.Swap` 必须保证：旧引用继续可用，新请求取新引用
- `Store.TryUpdate` 在 build 失败时不得替换 `current`

---

## 14. 自检清单

- [ ] 是否在 Lexicon API 引入了业务概念？
- [ ] `Entry(id)` 是否为 O(1)？
- [ ] Build 阶段是否一次性完成所有索引构建？
- [ ] 运行期是否修改了 Lexicon 内部结构？
- [ ] 热更新是否破坏了正在进行的请求？
- [ ] `All()` 顺序是否确定？
- [ ] 是否在 Runtime 做了 Build 才能完成的优化？
- [ ] 是否实现了 `LexiconSource` 抽象（而非直接喂 Builder）？
- [ ] Compose 时冲突是否返回 `ErrConflict` 而非静默覆盖？
- [ ] Last Known Good 是否在 build 失败时保留 V1？

---

## 15. 相关文档

- 上游：[02-核心领域模型](02-核心领域模型.md) §7, §8
- 状态读写：[05-State与保护区机制](05-State与保护区机制.md)
- 性能：[14-性能设计与算法优化](14-性能设计与算法优化.md)
- 高可用整体：[§34](07-Engine与Profile.md) / 1.2 规范 §34
- 测试：[15-测试策略与质量工程](15-测试策略与质量工程.md)
