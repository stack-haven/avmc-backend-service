package biz

import (
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ListOption 列表查询选项函数
type ListOption func(*ListOptions)

// ListOptions 列表查询参数
type ListOptions struct {
	Filter  filtering.Filter
	OrderBy ordering.OrderBy
	Offset  int
	Limit   int
}

// ListFilter 设置过滤条件
func ListFilter(filter filtering.Filter) ListOption {
	return func(o *ListOptions) {
		o.Filter = filter
	}
}

// ListOrderBy 设置排序
func ListOrderBy(orderBy ordering.OrderBy) ListOption {
	return func(o *ListOptions) {
		o.OrderBy = orderBy
	}
}

// ListOffset 设置偏移量
func ListOffset(offset int) ListOption {
	return func(o *ListOptions) {
		if offset < 0 {
			offset = 0
		}
		o.Offset = offset
	}
}

// ListLimit 设置条数限制
func ListLimit(limit int) ListOption {
	return func(o *ListOptions) {
		if limit <= 0 {
			limit = DefaultPageSize
		}
		if limit > MaxPageSize {
			limit = MaxPageSize
		}
		o.Limit = limit
	}
}

func NormalizePageSize(size int32) int {
	if size <= 0 {
		return DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return int(size)
}

// DefualtOrderBy 默认分页排序
func DefualtOrderBy(field ordering.Field) ordering.OrderBy {
	oby := ordering.OrderBy{}
	oby.Fields = append(oby.Fields, field)
	return oby
}
