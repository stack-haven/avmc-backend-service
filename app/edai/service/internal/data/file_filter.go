package data

import (
	"fmt"
	"time"

	"backend-service/app/edai/service/internal/data/ent/gen/filerecord"

	"entgo.io/ent/dialect/sql"
	"go.einride.tech/aip/filtering"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// fieldKind 是过滤字段的取值类型，用于在翻译阶段做类型校验。
type fieldKind int

const (
	kindString fieldKind = iota
	kindInt
	kindBool
	kindTime
)

// fieldSet 描述某个列表接口允许过滤的字段及其类型。
//
// 服务层用 filtering.DeclareIdent 声明同样的字段集合；两张表必须保持一致，
// 否则会出现「解析通过但翻译失败」的 400。新增字段时两处一起改。
type fieldSet map[string]fieldKind

// exprFilter 把过滤表达式翻译成 SQL 谓词。
//
// 为什么不用 aip-go 的 ents.ApplyFilter：它在翻译失败时**静默返回空条件**，
// 也就是「筛选条件写错了 → 悄悄返回全量」。对资产检索来说这比报错更危险
// （用户会以为筛选生效）。这里改为显式返回错误，由服务层转成 400。
type exprFilter func(s *sql.Selector) *sql.Predicate

// buildExprFilter 校验字段类型并返回可直接用于 ent 查询的 where 修饰器。
func buildExprFilter(filter filtering.Filter, fields fieldSet) (func(*sql.Selector), error) {
	if filter.CheckedExpr == nil || filter.CheckedExpr.GetExpr() == nil {
		return nil, nil
	}
	fn, err := exprToFilter(filter.CheckedExpr.GetExpr(), fields)
	if err != nil {
		return nil, err
	}
	return func(s *sql.Selector) { s.Where(fn(s)) }, nil
}

func exprToFilter(e *expr.Expr, fields fieldSet) (exprFilter, error) {
	switch kind := e.GetExprKind().(type) {
	case *expr.Expr_CallExpr:
		return callToFilter(kind.CallExpr, fields)
	case *expr.Expr_ConstExpr:
		if kind.ConstExpr.GetBoolValue() {
			return func(*sql.Selector) *sql.Predicate { return sql.ExprP("TRUE") }, nil
		}
		return func(*sql.Selector) *sql.Predicate { return sql.ExprP("FALSE") }, nil
	default:
		return nil, fmt.Errorf("不支持的过滤表达式: %T", kind)
	}
}

func callToFilter(call *expr.Expr_Call, fields fieldSet) (exprFilter, error) {
	switch call.GetFunction() {
	case filtering.FunctionAnd, filtering.FunctionFuzzyAnd:
		return combineFilters(call.GetArgs(), fields, sql.And)
	case filtering.FunctionOr:
		return combineFilters(call.GetArgs(), fields, sql.Or)
	case filtering.FunctionNot:
		if len(call.GetArgs()) != 1 {
			return nil, fmt.Errorf("NOT 需要 1 个参数")
		}
		inner, err := exprToFilter(call.GetArgs()[0], fields)
		if err != nil {
			return nil, err
		}
		return func(s *sql.Selector) *sql.Predicate { return sql.Not(inner(s)) }, nil
	default:
		return comparisonToFilter(call.GetFunction(), call.GetArgs(), fields)
	}
}

func combineFilters(args []*expr.Expr, fields fieldSet, combine func(...*sql.Predicate) *sql.Predicate) (exprFilter, error) {
	fns := make([]exprFilter, 0, len(args))
	for _, arg := range args {
		fn, err := exprToFilter(arg, fields)
		if err != nil {
			return nil, err
		}
		fns = append(fns, fn)
	}
	return func(s *sql.Selector) *sql.Predicate {
		preds := make([]*sql.Predicate, 0, len(fns))
		for _, fn := range fns {
			preds = append(preds, fn(s))
		}
		return combine(preds...)
	}, nil
}

func comparisonToFilter(function string, args []*expr.Expr, fields fieldSet) (exprFilter, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("%s 需要 2 个参数", function)
	}
	field, ok := identName(args[0])
	if !ok {
		return nil, fmt.Errorf("过滤条件左侧必须是字段名")
	}
	kind, ok := fields[field]
	if !ok {
		return nil, fmt.Errorf("不支持的过滤字段: %s", field)
	}
	value, err := constValue(args[1], kind)
	if err != nil {
		return nil, fmt.Errorf("字段 %s: %w", field, err)
	}

	switch function {
	case filtering.FunctionHas:
		// `file_name:"poster"` = 包含匹配，只对有意义的字符串字段开放。
		if kind != kindString {
			return nil, fmt.Errorf("字段 %s 不支持包含匹配（:）", field)
		}
		text := value.(string)
		return func(s *sql.Selector) *sql.Predicate {
			return sql.Contains(s.C(field), text)
		}, nil
	case filtering.FunctionEquals, filtering.FunctionNotEquals:
		fn := func(s *sql.Selector) *sql.Predicate {
			p := sql.EQ(s.C(field), value)
			// is_duplicate 实际是三态：真正重复 / 真正不重复 / 没算过哈希（不知道）。
			// 表里是 bool，未算哈希时恒为 false，直接 `= false` 会把「不知道」
			// 混进「不重复」，用户会以为这些文件确认过没有副本。
			// 因此两个方向都要求 content_hash 非空，只返回有结论的记录。
			if field == filerecord.FieldIsDuplicate {
				return sql.And(p, sql.NotNull(s.C(filerecord.FieldContentHash)))
			}
			return p
		}
		if function == filtering.FunctionEquals {
			return fn, nil
		}
		return func(s *sql.Selector) *sql.Predicate { return sql.Not(fn(s)) }, nil
	case filtering.FunctionLessThan, filtering.FunctionLessEquals,
		filtering.FunctionGreaterThan, filtering.FunctionGreaterEquals:
		// 只有数值与时间字段支持范围比较：字符串字段用包含匹配更符合预期。
		if kind != kindInt && kind != kindTime {
			return nil, fmt.Errorf("字段 %s 不支持范围比较", field)
		}
		return func(s *sql.Selector) *sql.Predicate {
			col := s.C(field)
			switch function {
			case filtering.FunctionLessThan:
				return sql.LT(col, value)
			case filtering.FunctionLessEquals:
				return sql.LTE(col, value)
			case filtering.FunctionGreaterThan:
				return sql.GT(col, value)
			default:
				return sql.GTE(col, value)
			}
		}, nil
	default:
		return nil, fmt.Errorf("不支持的过滤操作: %s", function)
	}
}

func identName(e *expr.Expr) (string, bool) {
	ident := e.GetIdentExpr()
	if ident == nil {
		return "", false
	}
	return ident.GetName(), true
}

// constValue 取出常量并按字段类型校验。
func constValue(e *expr.Expr, kind fieldKind) (any, error) {
	if call := e.GetCallExpr(); call != nil {
		// timestamp("2024-01-01T00:00:00Z") → time.Time
		if call.GetFunction() == filtering.FunctionTimestamp && len(call.GetArgs()) == 1 {
			raw := call.GetArgs()[0].GetConstExpr().GetStringValue()
			ts, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return nil, fmt.Errorf("时间值格式错误（需 RFC3339）: %s", raw)
			}
			if kind != kindTime {
				return nil, fmt.Errorf("该字段不是时间类型")
			}
			return ts, nil
		}
		return nil, fmt.Errorf("不支持的过滤值表达式: %s", call.GetFunction())
	}
	c := e.GetConstExpr()
	if c == nil {
		// `field=true` 里的 true/false 会被 einride 解析成标识符（不是常量），
		// 服务层已把这两个字面量声明为 bool，这里把它们还原成常量值。
		if ident := e.GetIdentExpr(); ident != nil {
			switch ident.GetName() {
			case "true":
				return true, nil
			case "false":
				return false, nil
			}
		}
		return nil, fmt.Errorf("过滤条件右侧必须是常量")
	}
	switch kind {
	case kindString:
		if s, ok := c.GetConstantKind().(*expr.Constant_StringValue); ok {
			return s.StringValue, nil
		}
	case kindInt:
		if n, ok := c.GetConstantKind().(*expr.Constant_Int64Value); ok {
			return n.Int64Value, nil
		}
	case kindBool:
		if b, ok := c.GetConstantKind().(*expr.Constant_BoolValue); ok {
			return b.BoolValue, nil
		}
	case kindTime:
		if s, ok := c.GetConstantKind().(*expr.Constant_StringValue); ok {
			ts, err := time.Parse(time.RFC3339, s.StringValue)
			if err != nil {
				return nil, fmt.Errorf("时间值格式错误（需 RFC3339）: %s", s.StringValue)
			}
			return ts, nil
		}
	}
	return nil, fmt.Errorf("值类型与字段类型不匹配")
}
