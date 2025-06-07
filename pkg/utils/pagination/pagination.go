package pagination

const (
	PAGE          = 1
	PAGE_SIZE     = 10
	MAX_PAGE_SIZE = 1000
)

func GetPageOffset(page, pageSize int32) int {
	return int((page - 1) * pageSize)
}

type Query struct {
	Key       string
	Condition string
	Args      []interface{}
}

type OrderBy struct {
	Column string
	Desc   bool
}

type PaginationHandler interface {
	GetPage() int32
	GetPageSize() int32
	GetPageOffset() int
	Reset() error
}
