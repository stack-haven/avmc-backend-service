# Entgo 分页封装器

这是一个为Entgo ORM框架提供的分页、排序和筛选功能的封装器，旨在简化后端API中的数据查询操作。

## 目录结构

```
/pkg/entgo/paging/
├── paging.go         # 核心实现文件
├── paging_test.go    # 单元测试
├── example_usage.go  # 使用示例
└── example_apply_option_func.go # MakeApplyOptionFunc使用示例
```

## 功能特性

1. **分页处理**：支持页码、页大小设置，自动计算偏移量
2. **排序功能**：支持多字段排序，支持升序和降序
3. **筛选条件**：支持JSON格式的查询条件，支持AND和OR逻辑
4. **字段掩码**：支持选择特定字段进行查询
5. **类型安全**：提供两种类型安全的使用模式

## 核心组件

### 1. PagingOption 结构体

封装了分页、排序和筛选所需的所有参数。

```go
// PagingOption 封装分页、排序和筛选参数
type PagingOption struct {
	Limit    int                     // 每页记录数
	Offset   int                     // 偏移量
	NoPaging bool                    // 是否禁用分页
	Order    []func(*sql.Selector)   // 排序函数列表
	Filters  []func(*sql.Selector)   // 过滤条件函数列表
	Fields   []string                // 要选择的字段列表
}
```

### 2. 转换函数

#### ConvertPagingRequest

将protobuf定义的PagingRequest转换为PagingOption。

```go
func ConvertPagingRequest(req *pagination.PagingRequest) *PagingOption {
	// 实现逻辑
}
```

#### NewPagingResponse

创建分页响应对象。

```go
func NewPagingResponse(total int32, _ interface{}) *pagination.PagingResponse {
	// 实现逻辑
}
```

### 3. 分页应用机制

由于Go泛型的局限性，我们提供了两种使用模式：

#### 模式1：直接使用类型断言（适合简单场景）

在调用处直接使用类型断言和具体的方法调用：

```go
// 获取分页请求
pagingReq, err := paging.ConvertPagingRequest(httpReq)
if err != nil {
	// 处理错误
	return
}

// 创建查询
query := gen.NewRoleQuery()

// 直接应用分页选项（在调用处）
if pagingReq != nil {
	// 应用过滤条件
	for _, filter := range pagingReq.Filters {
		query.Modify(filter)
	}

	// 应用排序
	for _, order := range pagingReq.Order {
		query.Order(order)
	}

	// 应用分页
	if !pagingReq.NoPaging {
		query.Limit(pagingReq.Limit).Offset(pagingReq.Offset)
	}
}

// 执行查询
roles, err := query.All(context.Background())
```

#### 模式2：使用MakeApplyOptionFunc（适合复杂场景）

创建类型安全的专用分页应用函数：

```go
// 为特定查询类型创建专用的ApplyOption函数
applyRoleQueryOption := paging.MakeApplyOptionFunc[*gen.RoleQuery](
	// 应用过滤条件
	func(query *gen.RoleQuery, filters []func(*sql.Selector)) *gen.RoleQuery {
		for _, filter := range filters {
			query.Modify(filter)
		}
		return query
	},
	// 应用排序
	func(query *gen.RoleQuery, orderBy []func(*sql.Selector)) *gen.RoleQuery {
		for _, order := range orderBy {
			query.Order(order)
		}
		return query
	},
	// 应用分页
	func(query *gen.RoleQuery, limit, offset int) *gen.RoleQuery {
		return query.Limit(limit).Offset(offset)
	},
)

// 使用专用函数应用分页选项
query := applyRoleQueryOption(gen.NewRoleQuery(), pagingReq)

// 执行查询
roles, err := query.All(context.Background())
```

### 4. 辅助函数

#### parseQueryFilter

解析JSON格式的查询条件，生成AND过滤函数。

```go
func parseQueryFilter(query string) (func(*sql.Selector), error) {
	// 实现逻辑
}
```

#### parseOrQueryFilter

解析JSON格式的或查询条件，生成OR过滤函数。

```go
func parseOrQueryFilter(query string) (func(*sql.Selector), error) {
	// 实现逻辑
}
```

#### ApplyFieldMask

将字段掩码应用到查询。

```go
func ApplyFieldMask(selector *sql.Selector, fields []string) bool {
	// 实现逻辑
}
```

## 使用示例

### 1. 基本分页查询

```go
func ListRoles(ctx context.Context, req *pagination.PagingRequest) (*pagination.PagingResponse, error) {
	// 转换分页请求
	opt := paging.ConvertPagingRequest(req)

	// 创建查询
	query := gen.NewRoleQuery()

	// 添加自定义过滤条件
	query.Where(gen.RoleAgeGT(18))

	// 应用分页选项（直接使用类型断言）
	if opt != nil {
		// 应用过滤条件
		for _, filter := range opt.Filters {
			query.Modify(filter)
		}

		// 应用排序
		for _, order := range opt.Order {
			query.Order(order)
		}

		// 应用分页
		if !opt.NoPaging {
			query.Limit(opt.Limit).Offset(opt.Offset)
		}
	}

	// 执行查询
	roles, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	// 查询总数
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	// 创建分页响应
	resp := paging.NewPagingResponse(int32(count), nil)

	// 注意：NewPagingResponse目前只设置了Total字段
	// 您需要根据具体的protobuf消息类型，正确设置Items字段
	// resp.Items = convertconvertProtoItems(roles)

	return resp, nil
}
```

### 2. 使用MakeApplyOptionFunc

```go
// 初始化时创建专用的ApplyOption函数
var applyRoleQueryOption = paging.MakeApplyOptionFunc[*gen.RoleQuery](
	func(query *gen.RoleQuery, filters []func(*sql.Selector)) *gen.RoleQuery {
		for _, filter := range filters {
			query.Modify(filter)
		}
		return query
	},
	func(query *gen.RoleQuery, orderBy []func(*sql.Selector)) *gen.RoleQuery {
		for _, order := range orderBy {
			query.Order(order)
		}
		return query
	},
	func(query *gen.RoleQuery, limit, offset int) *gen.RoleQuery {
		return query.Limit(limit).Offset(offset)
	},
)

func ListRolesWithDedicatedFunc(ctx context.Context, req *pagination.PagingRequest) (*pagination.PagingResponse, error) {
	// 转换分页请求
	opt := paging.ConvertPagingRequest(req)

	// 创建查询
	query := gen.NewRoleQuery()

	// 添加自定义过滤条件
	query.Where(gen.RoleAgeGT(18))

	// 使用专用函数应用分页选项
	query = applyRoleQueryOption(query, opt)

	// 执行查询
	roles, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	// 查询总数
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	// 创建分页响应
	resp := paging.NewPagingResponse(int32(count), nil)

	return resp, nil
}
```

## 设计理念

1. **类型安全**：避免使用反射，通过类型断言或专用函数实现类型安全
2. **灵活性**：提供多种使用模式，适应不同场景需求
3. **易用性**：简化分页、排序和筛选操作的代码复杂度
4. **性能优先**：避免不必要的性能开销，符合entgo的设计理念

## 注意事项

1. 目前的`ApplyOption`函数只是一个简化版本，不实际应用分页选项
2. `NewPagingResponse`函数目前只设置了Total字段，需要手动设置Items字段
3. 对于复杂的查询场景，建议使用`MakeApplyOptionFunc`创建专用的分页应用函数
4. 所有查询条件的字段名必须符合`isValidFieldName`函数的验证规则（只能包含字母、数字、下划线和点）