package service

import (
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"

	pb "backend-service/api/edai/service/v1"
)

// listRequest 是各列表接口共有的 AIP 列表参数。
//
// 三家的 generated request 都实现了这 4 个 getter，因此可以用同一个接口解析，
// 不必依赖 generated 之间的公共父类型。
type listRequest interface {
	GetFilter() string
	GetOrderBy() string
	GetPageSize() int32
	GetPageToken() string
}

const (
	// edaiDefaultPageSize 与 proto 中 page_size 的 default 保持一致。
	edaiDefaultPageSize = 20
	// edaiMaxPageSize 与 proto 校验一致（lte: 200）。
	//
	// 刻意不复用 pkg/aip/listing.ParseParams：它把上限压到 100，
	// 而 EDAI 的工作台会一次拉 200 条做统计，压到 100 会静默少算。
	edaiMaxPageSize = 200
)

// listParams 解析后的列表参数。
type listParams struct {
	PageSize int
	Offset   int
	Filter   filtering.Filter
	OrderBy  ordering.OrderBy
}

// parseListParams 解析 AIP-160 过滤、排序与分页。
//
// 设计要点：
//   - 过滤条件**先解析再执行**：声明过的 ident 才允许出现，未声明字段直接报错，
//     而不是静默忽略（静默忽略会让用户以为筛选生效了，实际返回全量）；
//   - 排序字段走白名单校验，避免拿任意列名去拼 SQL；
//   - page_token 沿用 EDAI 的数字 offset 约定（前端是页码式表格）。
func parseListParams(req listRequest, sortable []string, decls ...filtering.DeclarationOption) (listParams, error) {
	options := make([]filtering.DeclarationOption, 0, len(decls)+3)
	options = append(options, filtering.DeclareStandardFunctions())
	// AIP-160 里 true/false 是字面量，但 einride 的解析器把裸 true 当标识符处理，
	// 不声明就会报 "undeclared identifier 'true'"，导致 `is_duplicate=true`
	// 这类最自然的布尔筛选直接 400。这里显式声明成 bool 标识符。
	options = append(options,
		filtering.DeclareIdent("true", filtering.TypeBool),
		filtering.DeclareIdent("false", filtering.TypeBool),
	)
	options = append(options, decls...)
	declarations, err := filtering.NewDeclarations(options...)
	if err != nil {
		return listParams{}, pb.ErrorBadRequest("过滤条件声明无效: %v", err)
	}

	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return listParams{}, pb.ErrorBadRequest("过滤条件无效: %v", err)
	}

	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return listParams{}, pb.ErrorBadRequest("排序条件无效: %v", err)
	}
	if len(orderBy.Fields) > 0 {
		if err := orderBy.ValidateForPaths(sortable...); err != nil {
			return listParams{}, pb.ErrorBadRequest("%s", "不支持的排序字段")
		}
	}

	offset := 0
	if tok := req.GetPageToken(); tok != "" {
		if v, perr := parseOffset(tok); perr == nil {
			offset = v
		}
	}

	pageSize := int(req.GetPageSize())
	switch {
	case pageSize <= 0:
		pageSize = edaiDefaultPageSize
	case pageSize > edaiMaxPageSize:
		pageSize = edaiMaxPageSize
	}

	return listParams{
		PageSize: pageSize,
		Offset:   offset,
		Filter:   filter,
		OrderBy:  orderBy,
	}, nil
}
