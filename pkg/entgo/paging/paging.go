package paging

import (
	"encoding/json"
	"fmt"
	"strings"

	"backend-service/api/common/pagination"

	"entgo.io/ent/dialect/sql"
)

// PagingOption 封装分页、排序和筛选参数
// 这个结构体包含了处理分页查询所需的所有选项
// 字段说明：
// - Limit: 每页记录数
// - Offset: 偏移量
// - NoPaging: 是否禁用分页
// - Order: 排序函数列表
// - Filters: 过滤条件函数列表
// - Fields: 要选择的字段列表
type PagingOption struct {
	Limit    int
	Offset   int
	NoPaging bool
	Order    []func(*sql.Selector)
	Filters  []func(*sql.Selector)
	Fields   []string
}

// QueryIface 定义查询接口
// 注：此接口目前未被使用，保留以便将来扩展
type QueryIface[T any] interface {
	Limit(int) T
	Offset(int) T
	Order(...func(*sql.Selector)) T
	Modify(...func(*sql.Selector)) T
}

// WithPagination 为查询添加分页参数
// 参数:
//
//	q - 原始查询对象
//	page - 页码
//	pageSize - 每页记录数
//
// 返回值:
//
//	添加了分页参数的查询对象
func WithPagination[Q interface {
	Limit(int) Q
	Offset(int) Q
}](q Q, page int, pageSize int) Q {
	if page <= 0 {
		page = 1
	}
	q = q.Limit(pageSize).Offset(pageSize * (page - 1))
	return q
}

// ConvertPagingRequest 将 proto PagingRequest 转换为 PagingOption
// 参数:
//
//	req - proto 定义的分页请求
//
// 返回值:
//
//	封装了分页和筛选参数的 PagingOption
func ConvertPagingRequest(req *pagination.PagingRequest) *PagingOption {
	opt := &PagingOption{}

	// 设置默认值和处理分页参数
	if req == nil {
		opt.Limit = 10
		opt.Offset = 0
		return opt
	}

	// 处理页码和页大小
	page := int(req.GetPage())
	pageSize := int(req.GetPageSize())

	// 设置默认值
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	// 限制最大页大小，防止查询过多数据
	if pageSize > 100 {
		pageSize = 100
	}

	opt.NoPaging = req.GetNoPaging()
	if !opt.NoPaging {
		opt.Limit = pageSize
		opt.Offset = (page - 1) * pageSize
	} else {
		opt.Limit = 0
		opt.Offset = 0
	}

	// 处理字段掩码
	if mask := req.GetFieldMask(); mask != nil {
		opt.Fields = mask.GetPaths()
	}

	// 处理查询条件
	if query := req.GetQuery(); query != "" {
		if filter, err := parseQueryFilter(query); err == nil {
			opt.Filters = append(opt.Filters, filter)
		}
	}

	// 处理或查询条件
	if orQuery := req.GetOrQuery(); orQuery != "" {
		if filter, err := parseOrQueryFilter(orQuery); err == nil {
			opt.Filters = append(opt.Filters, filter)
		}
	}

	// 处理排序条件
	for _, ob := range req.GetOrderBy() {
		if ob == "" {
			continue
		}

		// 支持 proto 中定义的 "-field" 格式表示降序
		var field string
		var isDesc bool
		if strings.HasPrefix(ob, "-") {
			field = ob[1:]
			isDesc = true
		} else {
			// 也支持传统的 "field desc" 格式
			parts := strings.Fields(ob)
			switch len(parts) {
			case 1:
				field = parts[0]
			case 2:
				field = parts[0]
				dir := strings.ToLower(parts[1])
				isDesc = (dir == "desc")
			default:
				continue
			}
		}

		// 验证字段名格式
		if !isValidFieldName(field) {
			continue
		}

		// 创建排序函数
		if isDesc {
			opt.Order = append(opt.Order, func(s *sql.Selector) {
				s.OrderBy(sql.Desc(field))
			})
		} else {
			opt.Order = append(opt.Order, func(s *sql.Selector) {
				s.OrderBy(sql.Asc(field))
			})
		}
	}

	return opt
}

// 分页应用机制说明
//
// 由于Go泛型的局限性，我们无法创建一个适用于所有entgo查询类型的单一函数
// 因此，我们提供了两种使用模式：
// 1. 在调用处直接使用类型断言和具体方法调用（适合简单场景）
// 2. 使用MakeApplyOptionFunc创建类型安全的专用函数（适合复杂场景）

// ApplyOptionFunc 函数类型，用于将分页选项应用到具体的查询类型
// 参数:
//
//	query - 具体类型的查询对象（如*gen.RoleQuery）
//	opt - 分页选项
//
// 返回值:
//
//	应用了分页选项的查询对象
type ApplyOptionFunc[Q any] func(Q, *PagingOption) Q

// ApplyOption 简化的分页应用函数
// 注意：此函数仅返回原始查询对象，不实际应用分页选项
// 主要用于保持API兼容性，实际应用需要在调用处进行
// 参数:
//
//	query - ent查询对象
//	opt - 分页选项
//
// 返回值:
//
//	原始查询对象
func ApplyOption[Q any](query Q, _ *PagingOption) Q {
	// 此函数为了保持API兼容性而保留
	// 在实际项目中，建议使用类型断言或MakeApplyOptionFunc
	return query
}

// MakeApplyOptionFunc 创建适用于特定查询类型的分页应用函数
// 这是推荐的方法，可以创建类型安全的分页应用函数
// 参数:
//
//	applyFilters - 应用过滤条件的函数
//	applyOrder - 应用排序的函数
//	applyPaging - 应用分页(Limit/Offset)的函数
//
// 返回值:
//
//	一个类型安全的ApplyOption函数
func MakeApplyOptionFunc[Q any](
	applyFilters func(Q, []func(*sql.Selector)) Q,
	applyOrder func(Q, []func(*sql.Selector)) Q,
	applyPaging func(Q, int, int) Q,
) ApplyOptionFunc[Q] {
	return func(query Q, opt *PagingOption) Q {
		if opt == nil {
			return query
		}

		// 应用过滤条件
		if len(opt.Filters) > 0 && applyFilters != nil {
			query = applyFilters(query, opt.Filters)
		}

		// 应用排序
		if len(opt.Order) > 0 && applyOrder != nil {
			query = applyOrder(query, opt.Order)
		}

		// 应用分页
		if !opt.NoPaging && applyPaging != nil {
			query = applyPaging(query, opt.Limit, opt.Offset)
		}

		return query
	}
}

// parseQueryFilter 解析JSON格式的查询条件
// 参数:
//
//	query - JSON格式的查询字符串
//
// 返回值:
//
//	解析后的过滤函数和可能的错误
func parseQueryFilter(query string) (func(*sql.Selector), error) {
	// 解析JSON查询条件
	var filters map[string]interface{}
	if err := json.Unmarshal([]byte(query), &filters); err != nil {
		// 如果不是有效的JSON，尝试作为简单的文本搜索
		return func(s *sql.Selector) {
			// 这里可以根据实际需要实现简单的文本搜索逻辑
			// 例如搜索name和description字段
			text := strings.TrimSpace(query)
			if text != "" {
				like := fmt.Sprintf("%%%s%%", text)
				s.Where(sql.Or(sql.Like("name", like), sql.Like("description", like)))
			}
		}, nil
	}

	// 构建AND过滤条件
	return func(s *sql.Selector) {
		for field, value := range filters {
			if !isValidFieldName(field) {
				continue
			}

			// 根据值的类型构建不同的过滤条件
			switch v := value.(type) {
			case string:
				if v != "" {
					s.Where(sql.EQ(field, v))
				}
			case float64:
				s.Where(sql.EQ(field, v))
			case bool:
				s.Where(sql.EQ(field, v))
			case nil:
				s.Where(sql.IsNull(field))
			}
		}
	}, nil
}

// parseOrQueryFilter 解析JSON格式的或查询条件
// 参数:
//
//	query - JSON格式的查询字符串
//
// 返回值:
//
//	解析后的或过滤函数和可能的错误
func parseOrQueryFilter(query string) (func(*sql.Selector), error) {
	// 解析JSON查询条件
	var filters map[string]interface{}
	if err := json.Unmarshal([]byte(query), &filters); err != nil {
		return nil, err
	}

	// 构建OR过滤条件
	return func(s *sql.Selector) {
		var orPredicates []*sql.Predicate
		for field, value := range filters {
			if !isValidFieldName(field) {
				continue
			}

			// 根据值的类型构建不同的过滤条件
			switch v := value.(type) {
			case string:
				if v != "" {
					orPredicates = append(orPredicates, sql.EQ(field, v))
				}
			case float64:
				orPredicates = append(orPredicates, sql.EQ(field, v))
			case bool:
				orPredicates = append(orPredicates, sql.EQ(field, v))
			case nil:
				orPredicates = append(orPredicates, sql.IsNull(field))
			}
		}

		// 如果有OR条件，应用到查询中
		if len(orPredicates) > 0 {
			s.Where(sql.Or(orPredicates...))
		}
	}, nil
}

// ApplyFieldMask 将字段掩码应用到查询
// 参数:
//
//	selector - SQL 选择器
//	fields - 要选择的字段列表
//
// 返回值:
//
//	是否应用了字段掩码
func ApplyFieldMask(selector *sql.Selector, fields []string) bool {
	if selector == nil || len(fields) == 0 {
		return false
	}

	// 添加指定的字段
	for _, field := range fields {
		if isValidFieldName(field) {
			selector.Select(field)
		}
	}

	return true
}

// isValidFieldName 验证字段名是否有效
// 参数:
//
//	field - 字段名
//
// 返回值:
//
//	字段名是否有效
func isValidFieldName(field string) bool {
	// 简单验证，确保字段名只包含字母、数字和下划线
	for _, c := range field {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '.') {
			return false
		}
	}
	return field != ""
}

// NewPagingResponse 创建分页响应对象
// 参数:
//
//	total - 总记录数
//	items - 数据列表（任意类型，目前未使用）
//
// 返回值:
//
//	包含总记录数的分页响应对象
//
// 注意：此函数目前只设置了Total字段
// 在实际项目中，您需要根据具体的protobuf消息类型，正确设置Items字段
func NewPagingResponse(total int32, _ interface{}) *pagination.PagingResponse {
	return &pagination.PagingResponse{
		Total: total,
	}
}
