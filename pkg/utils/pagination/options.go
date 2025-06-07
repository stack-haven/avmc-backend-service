package pagination

import (
	"encoding/json"
	"log"
)

type Option func(*Pagination)

type Pagination struct {
	// 当前页
	Page int32
	// 每一页的行数
	PageSize int32
	// 查询参数
	Query map[string]interface{}
	// 排序
	OrderBy map[string]bool
	// 是否不分页
	Nopaging bool
}

func QueryUnmarshal(q map[string]string) map[string]string {
	if query, ok := q["query"]; ok {
		err := json.Unmarshal([]byte(query), &q)
		if err != nil {
			log.Println(err)
		}
	}
	return q
}

func OrderByUnmarshal(o map[string]bool) map[string]bool {
	// if order, ok := p.OrderBy["orderBy"]; ok {
	// 	err := json.Unmarshal([]byte(order), &p.OrderBy)
	// 	if err != nil {
	// 		log.Println(err)
	// 	}
	// }
	return o
}

func NewPagination(opts ...Option) *Pagination {
	p := Pagination{
		Page:     PAGE,
		PageSize: PAGE_SIZE,
		OrderBy:  make(map[string]bool),
		Query:    make(map[string]interface{}),
	}
	for _, o := range opts {
		o(&p)
	}
	if p.Nopaging {
		p.PageSize = MAX_PAGE_SIZE
	}
	return &p
}

func WithNopaging() Option {
	return func(p *Pagination) {
		p.Nopaging = true
		p.PageSize = MAX_PAGE_SIZE
	}
}

func WithPage(page int32) Option {
	return func(p *Pagination) {
		if page == 0 {
			page = PAGE
		}
		p.Page = page
	}
}

func WithPageSize(size int32) Option {
	return func(p *Pagination) {
		if size == 0 {
			size = PAGE_SIZE
		}
		p.PageSize = size
	}
}

func WithQuery(q map[string]interface{}) Option {
	return func(p *Pagination) {
		p.Query = q
	}
}

func WithOrderBy(o map[string]bool) Option {
	return func(p *Pagination) {
		p.OrderBy = o
	}
}
