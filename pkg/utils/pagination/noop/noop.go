package noop

import (
	"github.com/beiduoke/go-scaffold/pkg/util/pagination"
)

var _ pagination.PaginationHandler = (*noop)(nil)

type noop struct {
	paging pagination.Pagination
}

func NewPagination(opts ...pagination.Option) *noop {
	p := pagination.Pagination{
		Nopaging: false,
		Page:     pagination.PAGE,
		PageSize: pagination.PAGE_SIZE,
	}
	for _, o := range opts {
		o(&p)
	}
	return &noop{paging: p}
}

func (n *noop) GetPage() int32 {
	return n.paging.Page
}
func (n *noop) GetPageSize() int32 {
	return n.paging.PageSize
}
func (n *noop) GetPageOffset() int {
	return pagination.GetPageOffset(n.paging.Page, n.paging.PageSize)
}
func (n *noop) Reset() error {
	n = NewPagination()
	return nil
}
