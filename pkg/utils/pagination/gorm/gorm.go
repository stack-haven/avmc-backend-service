package gorm

import (
	"github.com/beiduoke/go-scaffold/pkg/util/pagination"
	"github.com/iancoleman/strcase"
)

var _ pagination.PaginationHandler = (*gormPagination)(nil)

type gormPagination struct {
	pageNum, pageSize int32
	nopaging          bool
	querys            []pagination.Query
	orders            []pagination.OrderBy
}

type Option func(*gormPagination)

func WithWhere(key string, condition string, args ...interface{}) Option {
	return func(p *gormPagination) {
		if condition == "" {
			condition = "="
		}
		p.SetQuery(pagination.Query{Key: key, Condition: condition, Args: args})
	}
}

func WithOrderBy(column string, desc bool) Option {
	return func(p *gormPagination) {
		p.SetOrderBy(pagination.OrderBy{Column: column, Desc: desc})
	}
}

func NewPagination(opts ...Option) *gormPagination {
	p := gormPagination{
		nopaging: false,
		pageNum:  pagination.PAGE,
		pageSize: pagination.PAGE_SIZE,
	}
	for _, o := range opts {
		o(&p)
	}
	return &p
}

func (p *gormPagination) String() string {
	return "GormPagination"
}

func (p *gormPagination) Reset() error {
	p.nopaging = false
	p.pageNum = pagination.PAGE
	p.pageSize = pagination.PAGE_SIZE
	p.querys = []pagination.Query{}
	p.orders = []pagination.OrderBy{}
	return nil
}

func (p *gormPagination) GetPageOffset() int {
	if p.nopaging {
		return 0
	}
	return pagination.GetPageOffset(p.pageNum, p.pageSize)
}

func (p *gormPagination) GetPage() int32 {
	return p.pageNum
}

func (p *gormPagination) GetPageSize() int32 {
	if p.nopaging {
		return pagination.MAX_PAGE_SIZE
	}
	return p.pageSize
}

func (p *gormPagination) GetNopaging() bool {
	return p.nopaging
}

func (p *gormPagination) GetQuerys() []pagination.Query {
	return p.querys
}

func (p *gormPagination) GetOrderBys() []pagination.OrderBy {
	return p.orders
}

func (p *gormPagination) SetQuery(condition ...pagination.Query) {
	for _, v := range condition {
		v.Key = strcase.ToSnakeWithIgnore(v.Key, " ")
		for k, vc := range p.querys {
			if v.Key == vc.Key {
				p.querys[k].Args = v.Args
				break
			}
		}
		p.querys = append(p.querys, v)
	}
}

func (p *gormPagination) SetOrderBy(order ...pagination.OrderBy) {
	for _, v := range order {
		v.Column = strcase.ToSnakeWithIgnore(v.Column, " ")
		for k, vc := range p.orders {
			if v.Column == vc.Column {
				p.orders[k].Desc = v.Desc
				break
			}
		}
		p.orders = append(p.orders, v)
	}
}

func (p *gormPagination) RemoveQuery(string) error {
	return nil
}
func (p *gormPagination) RemoveOrderBy(string) error {
	return nil
}
func (p *gormPagination) GetQuery(string) *pagination.Query {
	return nil
}
func (p *gormPagination) GetOrderBy(string) *pagination.OrderBy {
	return nil
}
